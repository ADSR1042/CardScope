import { useEffect, useState } from 'react';

import { formatDateTime, timeZoneLabel } from '../lib/time';

export function HeaderClock() {
  const [now, setNow] = useState(() => new Date());
  useEffect(() => {
    const update = () => setNow(new Date());
    const timer = window.setInterval(update, 1000);
    document.addEventListener('visibilitychange', update);
    return () => {
      window.clearInterval(timer);
      document.removeEventListener('visibilitychange', update);
    };
  }, []);
  const zone = timeZoneLabel(now);
  return (
    <div className="header-clock" title={`所有时间均按浏览器时区显示：${zone}`}>
      <time dateTime={now.toISOString()}>{formatDateTime(now)}</time>
      <span>{zone}</span>
    </div>
  );
}
