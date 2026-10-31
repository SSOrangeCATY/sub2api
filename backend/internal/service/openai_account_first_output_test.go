//go:build unit

package service

import (
 "bytes"
 "context"
 "errors"
 "fmt"
 "io"
 "net/http"
 "net/http/httptest"
 "strings"
 "sync/atomic"
 "testing"
 "time"
 "github.com/gin-gonic/gin"
 "github.com/stretchr/testify/require"
)

type accountFirstOutputHTTP struct { HTTPUpstream; client *http.Client }
func (u accountFirstOutputHTTP) Do(r *http.Request, _ string, _ int64, _ int) (*http.Response,error) { return u.client.Do(r) }

func TestAccountFirstOutputHungUpstream(t *testing.T) {
 for _, mode := range []string{"headers","sse-preamble","json-body"} {
  t.Run(mode,func(t *testing.T) {
   server:=httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter,r *http.Request){
    if mode=="sse-preamble" { w.Header().Set("Content-Type","text/event-stream"); fmt.Fprint(w,"data: {\"choices\":[{\"delta\":{\"role\":\"assistant\"}}]}\n\n: heartbeat\n\n"); w.(http.Flusher).Flush() }
    if mode=="json-body" {w.Header().Set("Content-Type","application/json"); w.WriteHeader(200); w.(http.Flusher).Flush()}
    <-r.Context().Done()
   }));defer server.Close()
   request,_:=http.NewRequest(http.MethodPost,server.URL+"/v1/chat/completions",nil)
   start:=time.Now()
   response,err:=doOpenAIWithAccountFirstOutputTimeout(request,50*time.Millisecond,server.Client().Do)
   require.Nil(t,response);require.ErrorIs(t,err,errOpenAIAccountFirstOutputTimeout)
   require.NotErrorIs(t,err,context.Canceled);require.NoError(t,request.Context().Err())
   require.Less(t,time.Since(start),time.Second)
  })
 }
}

func TestAccountFirstOutputHealthyStreamOutlivesBudget(t *testing.T) {
 prefix:="data: {\"choices\":[{\"delta\":{\"role\":\"assistant\"}}]}\r\n\r\ndata: {\"choices\":[{\"delta\":{\"content\":\"ok\"}}]}\r\n\r\n"
 suffix:="data: [DONE]\r\n\r\n"
 server:=httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter,r *http.Request){
  w.Header().Set("Content-Type","text/event-stream");fmt.Fprint(w,prefix);w.(http.Flusher).Flush()
  time.Sleep(160*time.Millisecond);fmt.Fprint(w,suffix)
 }));defer server.Close()
 request,_:=http.NewRequest(http.MethodPost,server.URL+"/v1/chat/completions",nil)
 response,err:=doOpenAIWithAccountFirstOutputTimeout(request,80*time.Millisecond,server.Client().Do)
 require.NoError(t,err);defer response.Body.Close()
 body,err:=io.ReadAll(response.Body);require.NoError(t,err);require.Equal(t,prefix+suffix,string(body))
}

func TestAccountFirstOutputReplayAndBounds(t *testing.T) {
 for _, wire:=range []string{
  "data: {\"choices\":[{\"delta\":{\"content\":\"ok\"}}]}\n\ndata: [DONE]\n\n",
  "data: {\"type\":\"response.output_text.delta\",\ndata: \"delta\":\"ok\"}\n\n: tail\n\n",
 } {
  resp:=&http.Response{Header:http.Header{"Content-Type":[]string{"text/event-stream"}},Body:io.NopCloser(strings.NewReader(wire))}
  prefix,err:=stageOpenAIAccountFirstOutput(resp);require.NoError(t,err)
  tail,err:=io.ReadAll(resp.Body);require.NoError(t,err);require.Equal(t,wire,string(prefix)+string(tail))
 }
 resp:=&http.Response{Header:http.Header{"Content-Type":[]string{"text/event-stream"}},Body:io.NopCloser(strings.NewReader(strings.Repeat("x",openAIFirstOutputStageMaxBytes+1)))}
 _,err:=stageOpenAIAccountFirstOutput(resp);require.ErrorIs(t,err,errOpenAIFirstOutputStageLimit)
}

func TestAccountFirstOutputClientCancel(t *testing.T) {
 server:=httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter,r *http.Request){<-r.Context().Done()}));defer server.Close()
 ctx,cancel:=context.WithCancel(context.Background());time.AfterFunc(20*time.Millisecond,cancel)
 req,_:=http.NewRequestWithContext(ctx,http.MethodPost,server.URL+"/v1/responses",nil)
 _,err:=doOpenAIWithAccountFirstOutputTimeout(req,time.Second,server.Client().Do)
 require.ErrorIs(t,err,context.Canceled);require.NotErrorIs(t,err,errOpenAIAccountFirstOutputTimeout)
}

func TestAccountFirstOutputAutomaticFailover(t *testing.T) {
 gin.SetMode(gin.TestMode)
 var calls atomic.Int32
 server:=httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter,r *http.Request){
  calls.Add(1)
  if r.Header.Get("Authorization")=="Bearer hung" {
   w.Header().Set("Content-Type","text/event-stream");fmt.Fprint(w,": heartbeat\n\n");w.(http.Flusher).Flush();<-r.Context().Done();return
  }
  w.Header().Set("Content-Type","application/json")
  fmt.Fprint(w,"{\"id\":\"ok\",\"object\":\"chat.completion\",\"model\":\"gpt-5.4\",\"choices\":[{\"index\":0,\"message\":{\"role\":\"assistant\",\"content\":\"fallback-ok\"},\"finish_reason\":\"stop\"}],\"usage\":{\"prompt_tokens\":2,\"completion_tokens\":2,\"total_tokens\":4}}")
 }));defer server.Close()
 svc:=&OpenAIGatewayService{cfg:rawChatCompletionsTestConfig(),httpUpstream:accountFirstOutputHTTP{client:server.Client()}}
 body:=[]byte("{\"model\":\"gpt-5.4\",\"messages\":[{\"role\":\"user\",\"content\":\"hello\"}],\"stream\":false}")
 rec:=httptest.NewRecorder();c,_:=gin.CreateTestContext(rec);c.Request=httptest.NewRequest(http.MethodPost,"/v1/chat/completions",bytes.NewReader(body))
 account:=rawChatCompletionsTestAccount();account.Credentials["base_url"]=server.URL;account.Credentials["api_key"]="hung";account.Extra=map[string]any{openAIAccountFirstOutputTimeoutKey:1}
 _,err:=svc.forwardAsRawChatCompletions(context.Background(),c,account,body,"")
 var failover *UpstreamFailoverError
 require.True(t,errors.As(err,&failover));require.False(t,c.Writer.Written());require.Zero(t,rec.Body.Len())
 account.ID++;account.Credentials["api_key"]="healthy"
 result,err:=svc.forwardAsRawChatCompletions(context.Background(),c,account,body,"")
 require.NoError(t,err);require.NotNil(t,result);require.Contains(t,rec.Body.String(),"fallback-ok");require.Equal(t,int32(2),calls.Load())
}

func TestAccountFirstOutputOptIn(t *testing.T) {
 req,_:=http.NewRequest(http.MethodPost,"https://example.com/v1/chat/completions",nil)
 account:=&Account{Platform:PlatformOpenAI,Type:AccountTypeAPIKey,Extra:map[string]any{}}
 require.Zero(t,openAIAccountFirstOutputTimeout(req,account))
 for _, v:=range []any{float64(6),6,"6"} {account.Extra[openAIAccountFirstOutputTimeoutKey]=v;require.Equal(t,6*time.Second,openAIAccountFirstOutputTimeout(req,account))}
 for _,v:=range []any{-1,0,301,"NaN","inf","bad"} {account.Extra[openAIAccountFirstOutputTimeoutKey]=v;require.Zero(t,openAIAccountFirstOutputTimeout(req,account))}
 account.Extra[openAIAccountFirstOutputTimeoutKey]=6;req.URL.Path="/v1/images/generations";require.Zero(t,openAIAccountFirstOutputTimeout(req,account))
}

func TestAccountFirstOutputTruncatedPreambleFailsBeforeWrite(t *testing.T) {
 gin.SetMode(gin.TestMode)
 for _, wire:=range []string{"", ": heartbeat\n\n", "data: {\"choices\":[{\"delta\":{\"role\":\"assistant\"}}]}\n\n"} {
  // wire is decoded below to obtain actual SSE delimiters.
  wire = strings.ReplaceAll(strings.ReplaceAll(wire, "\\n", "\n"), "\\\"", "\"")
  server:=httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter,r *http.Request){ w.Header().Set("Content-Type","text/event-stream");fmt.Fprint(w,wire) }))
  svc:=&OpenAIGatewayService{cfg:rawChatCompletionsTestConfig(),httpUpstream:accountFirstOutputHTTP{client:server.Client()}}
  account:=rawChatCompletionsTestAccount();account.Credentials["base_url"]=server.URL;account.Extra=map[string]any{openAIAccountFirstOutputTimeoutKey:1}
  body:=[]byte("{\"model\":\"gpt-5.4\",\"messages\":[{\"role\":\"user\",\"content\":\"hello\"}],\"stream\":true}")
  rec:=httptest.NewRecorder();c,_:=gin.CreateTestContext(rec);c.Request=httptest.NewRequest(http.MethodPost,"/v1/chat/completions",bytes.NewReader(body))
  _,err:=svc.forwardAsRawChatCompletions(context.Background(),c,account,body,"");var failover *UpstreamFailoverError
  require.True(t,errors.As(err,&failover));require.False(t,c.Writer.Written());require.Zero(t,rec.Body.Len())
  server.Close()
 }
}

func TestAccountFirstOutputStructuralToolStartDisarms(t *testing.T) {
 for _, kind:=range []string{"function_call", "custom_tool_call"} {
  wire:=fmt.Sprintf("data: {\"type\":\"response.output_item.added\",\"item\":{\"type\":\"%s\",\"arguments\":\"\",\"input\":\"\"}}\n\n",kind)
  server:=httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter,r *http.Request){w.Header().Set("Content-Type","text/event-stream");fmt.Fprint(w,wire);w.(http.Flusher).Flush();time.Sleep(120*time.Millisecond);fmt.Fprint(w,"data: [DONE]\n\n")}))
  req,_:=http.NewRequest(http.MethodPost,server.URL+"/v1/responses",nil)
  resp,err:=doOpenAIWithAccountFirstOutputTimeout(req,60*time.Millisecond,server.Client().Do)
  require.NoError(t,err);_,err=io.ReadAll(resp.Body);require.NoError(t,err);resp.Body.Close();server.Close()
 }
}
