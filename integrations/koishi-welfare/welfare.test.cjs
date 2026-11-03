const {test}=require('node:test');
const assert=require('node:assert/strict');
const {eventFor}=require('./index.cjs');
const {buildHtml,WelfareRenderer}=require('./report.cjs');
const session={platform:'qq',isDirect:false,userId:'actor',channelId:'channel',guildId:'group',messageId:'incoming-message'};
test('uses platform event identity and rejects private messages and invalid product IDs',()=>{
 assert.deepEqual(eventFor(session,'exchange','12'),{action:'exchange',group_id:'group',actor_id:'actor',message_id:'incoming-message',product_id:12});
 for(const other of [{...session,platform:'discord'},{...session,isDirect:true},{...session,messageId:''}])assert.throws(()=>eventFor(other,'checkin'));
 for(const id of ['-1','1.1','0','1;DROP'])assert.throws(()=>eventFor(session,'exchange',id));
});
test('HTML escapes product and receipt content, validates points, and contains no external assets',()=>{
 const snapshot={points:2,checked_in:false,products:[{id:1,title:'<script>alert(1)</script>',points_cost:1,balance_value:1,stock:2}],orders:[{id:'receipt',title:'<receipt>&"\'',balance_value:1}],generated_at:'2026-11-03T06:00:00Z',group_number:'487375322'};
 const html=buildHtml(snapshot);assert.ok(html.includes('&lt;script&gt;'));assert.ok(html.includes('&lt;receipt&gt;&amp;&quot;&#39;'));assert.ok(!html.includes('<script>'));assert.ok(html.includes("default-src 'none'"));assert.ok(!/https?:\/\//.test(html));
 assert.throws(()=>buildHtml({...snapshot,points:-1}));assert.throws(()=>buildHtml({...snapshot,points:1.1}));
});
test('personalized rendering never reuses another user result and recovers after failure',async()=>{
 const renderer=new WelfareRenderer();const calls=[];
 renderer._render=async snapshot=>{calls.push(snapshot.points);await new Promise(r=>setTimeout(r,5));if(snapshot.points===3)throw new Error('fixture failure');return Buffer.from(String(snapshot.points));};
 const results=await Promise.all([renderer.render({points:1}),renderer.render({points:2})]);assert.deepEqual(results.map(x=>x.toString()),['1','2']);
 await assert.rejects(renderer.render({points:3}));assert.equal((await renderer.render({points:4})).toString(),'4');assert.deepEqual(calls,[1,2,3,4]);await renderer.close();await assert.rejects(renderer.render({points:5}));
});
