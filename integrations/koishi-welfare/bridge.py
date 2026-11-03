"""Private, fixed-purpose transport. Only the host process can read the bot key."""
import json
import os
from pathlib import Path
import socketserver
from http.server import BaseHTTPRequestHandler
import subprocess
import urllib.request
import urllib.error

SOCKET = Path('/opt/koishi/data/welfare-center/bridge.sock')

def upstream(event):
    query = "BEGIN READ ONLY; SET LOCAL statement_timeout='3s'; SELECT bot_key FROM qq_welfare_config WHERE id=1; COMMIT;"
    result = subprocess.run(['docker','exec','-i','sub2api-postgres','psql','-X','-qAt','-v','ON_ERROR_STOP=1','-U','sub2api','-d','sub2api'],input=query,text=True,capture_output=True,timeout=5)
    if result.returncode or len(result.stdout.strip()) != 64:
        raise RuntimeError('welfare_key_unavailable')
    key = result.stdout.strip()
    container = json.loads(subprocess.check_output(['docker','inspect','sub2api'],text=True,timeout=2))[0]
    ip = next(v['IPAddress'] for v in container['NetworkSettings']['Networks'].values() if v.get('IPAddress'))
    request = urllib.request.Request('http://' + ip + ':8080/api/v1/qq-welfare/bot',data=json.dumps(event).encode(),headers={'Content-Type':'application/json','X-Welfare-Bot-Key':key},method='POST')
    opener = urllib.request.build_opener(urllib.request.ProxyHandler({}))
    try:
        response = opener.open(request,timeout=10)
    except urllib.error.HTTPError as error:
        response = error
    with response:
        body = response.read(512*1024+1)
        if len(body)>512*1024:
            raise RuntimeError('response_too_large')
        payload = json.loads(body)
        if response.status == 200 and payload.get('code') == 0:
            return 200, payload
        if str(payload.get('reason','')).startswith('WELFARE_'):
            return response.status, {'code':response.status,'reason':payload['reason'],'message':str(payload.get('message','福利服务暂不可用。'))[:500]}
        raise RuntimeError('welfare_upstream_unavailable')

class Handler(BaseHTTPRequestHandler):
    def setup(self):
        super().setup()
        self.connection.settimeout(20)

    def do_POST(self):
        if self.path != '/event':
            self.send_error(404)
            return
        try:
            length = int(self.headers.get('Content-Length','0'))
            if not 0 < length <= 4096:
                self.send_error(413)
                return
            event = json.loads(self.rfile.read(length))
            if not isinstance(event,dict) or set(event)-{'action','group_id','actor_id','message_id','code','product_id'}:
                self.send_error(400)
                return
            status,payload = upstream(event)
        except Exception:
            status,payload = 503,{'code':503,'message':'福利服务暂不可用，请稍后重试。'}
        body = json.dumps(payload,ensure_ascii=False).encode()
        self.send_response(status)
        self.send_header('Content-Type','application/json; charset=utf-8')
        self.send_header('Content-Length',str(len(body)))
        self.send_header('Cache-Control','no-store')
        self.end_headers()
        self.wfile.write(body)

    def log_message(self,*_):
        pass

if __name__ == '__main__':
    os.umask(0o077)
    SOCKET.parent.mkdir(mode=0o700,parents=True,exist_ok=True)
    SOCKET.unlink(missing_ok=True)
    with socketserver.UnixStreamServer(str(SOCKET),Handler) as server:
        SOCKET.chmod(0o600)
        server.serve_forever()
