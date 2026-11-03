const fs = require('node:fs');
const path = require('node:path');
const puppeteer = require('puppeteer-core');
const esc = value => String(value ?? '').replace(/[&<>"']/g, ch => ({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[ch]));
let font;
function buildHtml(snapshot) {
  if (!snapshot || !Number.isSafeInteger(snapshot.points) || snapshot.points < 0 || !Array.isArray(snapshot.products) || snapshot.products.length > 50 || !Array.isArray(snapshot.orders)) throw new Error('INVALID_WELFARE_SNAPSHOT');
  font ||= fs.readFileSync(path.join(__dirname, 'assets/Saira.ttf')).toString('base64');
  const date = new Intl.DateTimeFormat('zh-CN', {timeZone:'Asia/Shanghai',month:'2-digit',day:'2-digit',hour:'2-digit',minute:'2-digit',hourCycle:'h23'}).format(new Date(snapshot.generated_at));
  const products = snapshot.products.map(p => `<article class="product"><span class="id">${esc(p.id)}</span><div><h3>${esc(p.title)}</h3><span class="muted">余额 $${esc(p.balance_value)}</span></div><strong>${esc(p.points_cost)}<small> 积分</small></strong><span class="stock">库存 ${esc(p.stock)}</span></article>`).join('');
  const orders = snapshot.orders.slice(0,5).map(o => `<article class="order"><span>${esc(o.title)}</span><strong>+$${esc(o.balance_value)}</strong><span class="muted">${esc(o.id).slice(-8)}</span></article>`).join('');
  return `<!doctype html><html lang="zh-CN"><head><meta charset="utf-8"><meta http-equiv="Content-Security-Policy" content="default-src 'none'; style-src 'unsafe-inline'; font-src data:"><style>
  @font-face{font-family:Saira;src:url(data:font/ttf;base64,${font}) format('truetype');font-weight:100 900;font-stretch:50% 125%}
  *{box-sizing:border-box}body{margin:0;width:960px;font-family:'Saira','Noto Sans CJK SC',sans-serif;background:#F4F4F2;color:#161616}#report{width:960px;padding:0 34px 26px}.tri{height:4px;display:grid;grid-template-columns:1fr 1fr 1fr;margin:0 -34px}.tri i:nth-child(1){background:#ff00f0}.tri i:nth-child(2){background:#FFFA00}.tri i:nth-child(3){background:#00ffa2}
  header{display:flex;align-items:center;justify-content:space-between;padding:24px 0;border-bottom:2px solid #161616}.brand{font-size:14px;letter-spacing:2px;font-stretch:125%;font-weight:600;color:#5c5c5c}h1{font-size:38px;margin:4px 0 0;line-height:1.3}.timestamp{font-size:17px;color:#5c5c5c}.points{display:flex;align-items:center;justify-content:space-between;padding:24px 0;border-bottom:1px solid #d9d9d6}.points label{font-size:18px}.points strong{display:block;font-size:76px;font-weight:700;line-height:1.1;font-variant-numeric:tabular-nums}.points strong small{font-size:23px;font-weight:400}.checkin{font-size:21px;padding:10px 15px;background:${snapshot.checked_in ? '#161616' : '#FFFA00'};color:${snapshot.checked_in ? '#F4F4F2' : '#161616'}}
  .section{display:flex;align-items:center;justify-content:space-between;padding:22px 0 12px}h2{font-size:23px;margin:0}.section span{font-size:15px;color:#5c5c5c}.product{display:grid;grid-template-columns:42px minmax(0,1fr) 135px 105px;gap:14px;align-items:center;padding:16px 0;border-top:1px solid #d9d9d6}.id{font-size:22px;color:#5c5c5c;font-weight:600}h3{font-size:21px;margin:0 0 4px;overflow-wrap:anywhere}.muted{font-size:15px;color:#5c5c5c}.product strong{font-size:28px;white-space:nowrap;text-align:right}.product strong small{font-size:15px;font-weight:400}.stock{text-align:right;font-size:16px;color:#5c5c5c}.empty{padding:25px 0;border-top:1px solid #d9d9d6;font-size:18px;color:#5c5c5c}.order{display:grid;grid-template-columns:minmax(0,1fr) 120px 105px;gap:15px;padding:12px 0;border-top:1px solid #d9d9d6;font-size:18px;align-items:center}.order>span:first-child{overflow-wrap:anywhere}.order strong{text-align:right}.order .muted{text-align:right}.commands{margin-top:20px;border-top:2px solid #161616;padding-top:15px;font-size:18px;line-height:2}.commands b{display:inline-block;margin-right:18px}.footer{display:flex;justify-content:space-between;font-size:14px;color:#5c5c5c;padding-top:16px;border-top:1px solid #d9d9d6;margin-top:15px}
  </style></head><body><main id="report"><div class="tri"><i></i><i></i><i></i></div><header><div><div class="brand">// WELFARE CENTER</div><h1>福利中心</h1></div><span class="timestamp">${date} 更新</span></header>
  <section class="points"><div><label>我的积分</label><strong>${snapshot.points}<small> 积分</small></strong></div><div class="checkin">${snapshot.checked_in ? '今日已签到' : '福利签到 / +1 积分'}</div></section>
  <div class="section"><h2>积分商品</h2><span>福利兑换 商品编号</span></div>${products || '<div class="empty">管理员尚未上架积分商品。</div>'}
  <div class="section"><h2>最近领取</h2><span>订单尾号</span></div>${orders || '<div class="empty">还没有领取记录。</div>'}
  <div class="commands"><b>福利签到</b>每日 +1 积分<br><b>福利兑换 编号</b>兑换商品，余额自动到账<br><b>福利领取 兑换码</b>领取共享福利，每账号限一次</div><div class="footer"><span>AI.PTCRYS.NET / 小橘子</span><span>认证群 ${esc(snapshot.group_number)} / 北京时间</span></div></main></body></html>`;
}
class WelfareRenderer {
  constructor() { this.browser = null; this.queue = Promise.resolve(); this.disposed = false; }
  render(snapshot) {
    if(this.disposed) return Promise.reject(new Error('DISPOSED'));
    const job = this.queue.then(() => this._render(snapshot));
    this.queue = job.catch(() => {});
    return job;
  }
  async _render(snapshot) {
    if (!this.browser?.connected) this.browser = await puppeteer.launch({executablePath:'/usr/bin/chromium',headless:true,timeout:15000,args:['--no-sandbox','--disable-setuid-sandbox','--disable-dev-shm-usage','--disable-background-networking','--no-first-run']});
    const page=await this.browser.newPage();
    try{
      await page.setViewport({width:960,height:800});await page.setJavaScriptEnabled(false);await page.setRequestInterception(true);
      page.on('request',r=>r.url().startsWith('data:')?r.continue():r.abort());
      await page.setContent(buildHtml(snapshot),{waitUntil:'load',timeout:12000});await page.evaluate(()=>document.fonts.ready);
      const height=await page.$eval('#report',el=>el.scrollHeight);if(height>12000)throw new Error('REPORT_TOO_LARGE');
      const png=Buffer.from(await(await page.$('#report')).screenshot({type:'png'}));if(png.length>4*1024*1024)throw new Error('REPORT_TOO_LARGE');return png;
    }finally{await page.close()}
  }
  async close(){this.disposed=true;await this.queue;await this.browser?.close()}
}
module.exports={buildHtml,WelfareRenderer};
