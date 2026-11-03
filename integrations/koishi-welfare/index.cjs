const http = require('node:http');
const { h } = require('koishi');
const { WelfareRenderer } = require('./report.cjs');

function eventFor(session, action, argument = '') {
  if (session?.platform !== 'qq' || session.isDirect || !session.userId || !session.messageId || !session.channelId) {
    throw new Error('请在认证 QQ 群内 @小橘子 使用福利命令。');
  }
  const code = String(argument).trim();
  if (code.length > 64) throw new Error('命令参数过长。');
  if (!['points', 'checkin', 'verify', 'bind-group'].includes(action)) throw new Error('兑换和领取请前往网站福利中心。');
  return {
    action, group_id: session.guildId || session.channelId,
    actor_id: session.userId, message_id: session.messageId,
    code,
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
    ['福利签到', 'checkin', '每日签到获得 1 积分'],
    ['福利积分', 'points', '查看个人积分与签到状态'],
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
        if (result.points) return h.image(await renderer.render(result.points), 'image/png');
        return result.message || '福利服务暂不可用，请稍后重试。';
      } catch {
        logger.warn('Welfare request or image generation unavailable');
        return '福利服务暂不可用，请稍后查询「福利积分」。';
      }
    });
  }
  ctx.on('dispose', () => renderer.close());
};
