const fs=require('node:fs');
const {WelfareRenderer}=require('./report.cjs');
// Synthetic fixture for visual verification only; never sent to QQ or credited to an account.
const snapshot={points:12,checked_in:true,group_number:'487375322',generated_at:'2026-11-03T06:00:00Z',products:[{id:1,title:'签到余额福利',points_cost:5,balance_value:1,stock:20},{id:2,title:'限时积分兑换',points_cost:10,balance_value:3,stock:5}],orders:[{id:'00000000000000000000000012345678',title:'群友见面礼',balance_value:0.5}]};
(async()=>{const renderer=new WelfareRenderer();try{const image=await renderer.render(snapshot);fs.writeFileSync('/koishi/plugins/welfare-center/preview.png',image);console.log(JSON.stringify({bytes:image.length,fixture:true}));}finally{await renderer.close();}})().catch(()=>{console.error('Preview generation failed');process.exitCode=1});
