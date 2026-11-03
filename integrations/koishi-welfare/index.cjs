const http = require('node:http');
const { h } = require('koishi');
const { WelfareRenderer } = require('./report.cjs');

function eventFor(session, action, argument = '') {
  if (session?.platform !== 'qq' || session.isDirect || !session.userId || !session.messageId || !session.channelId) {
    throw new Error('请在认证 QQ 群内 @小橘子 使用福利命令。');
  }
  const code = String(argument).trim();
  if (code.length > 64) throw new Error('命令参数过长。');
  if (action === 'exchange' && !/^[1-9]\d{0,9}$/.test(code)) throw new Error('请输入商品编号，例如：福利兑换 1');
  return {
    action, group_id: session.guildId || session.channelId,
    actor_id: session.userId, message_id: session.messageId,
    ...(action === 'exchange' ? { product_id: Number(code) } : { code }),
  };
}

function bridge(event, socketPath = '/koishi/welfare-center/bridge.sock') {
  return new Promise((resolve, reject) => {
    const body = Buffer.from(JSON.stringify(event));
    const request = http.request({ socketPath, path: '/event', method: 'POST', timeout: 20000,
      headers: { 'Content-Type': 'application/json', 'Content-Length': body.length } }, response => {
      const chunks = []; let size = 0;
      response.on('data', chunk => {
        size += chunk.length;
        if (size > 512 * 1024) { response.destroy(new Error('RESPONSE_TOO_LARGE')); return; }
        chunks.push(chunk);
      });
      response.on('error', reject);
      response.on('end', () => {
        try {
          const data = JSON.parse(Buffer.concat(chunks).toString());
          if (response.statusCode !== 200 || data.code !== 0) {
            resolve({ message: /^WELFARE_/.test(data.reason || '') ? String(data.message) : '福利服务暂不可用，请稍后重试。' });
          } else resolve(data.data);
        } catch { reject(new Error('INVALID_RESPONSE')); }
      });
    });
    request.on('error', reject);
    request.on('timeout', () => request.destroy(new Error('BRIDGE_TIMEOUT')));
    request.end(body);
  });
}

exports.name = 'welfare-center';
exports.eventFor = eventFor;
exports.bridge = bridge;
exports.apply = ctx => {
  const renderer = new WelfareRenderer();
  const logger = ctx.logger('welfare-center');
  const commands = [
    ['福利中心', 'overview', '查看积分与积分商品'],
    ['福利签到', 'checkin', '每日签到获得 1 积分'],
    ['福利积分', 'overview', '查看个人福利积分'],
    ['福利记录', 'records', '查看最近领取记录'],
    ['福利兑换 <id:string>', 'exchange', '用积分兑换指定商品'],
    ['福利领取 <code:string>', 'claim', '领取共享兑换码，每账号限一次'],
    ['福利认证 <code:string>', 'verify', '使用网站生成的认证码绑定账号'],
    ['福利连接 <code:string>', 'bind-group', '管理员连接认证群'],
  ];
  for (const [name, action, description] of commands) {
    ctx.command(name, description).action(async ({ session }, argument) => {
      let event;
      try { event = eventFor(session, action, argument); }
      catch (error) { return error.message; }
      try {
        const result = await bridge(event);
        if (result.snapshot) return h.image(await renderer.render(result.snapshot), 'image/png');
        return result.message || '福利服务暂不可用，请稍后重试。';
      } catch {
        logger.warn('Welfare request or image generation unavailable');
        return '福利服务暂不可用。若刚执行了签到或兑换，请查询「福利记录」，勿重复购买。';
      }
    });
  }
  ctx.on('dispose', () => renderer.close());
};
