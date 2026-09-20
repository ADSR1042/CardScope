import { useState } from 'react';
import { Check, Copy } from 'lucide-react';
import { Button } from '../../components/ui/button';

export function Enrollment({ code }: { code: string }) {
  const [copied, setCopied] = useState(false);
  return (
    <div className="enrollment">
      <label>一次性接入码</label>
      <code>{code}</code>
      <Button
        variant="outline"
        onClick={async () => {
          try {
            await navigator.clipboard.writeText(code);
            setCopied(true);
          } catch {
            setCopied(false);
          }
        }}
      >
        {copied ? <Check size={15} /> : <Copy size={15} />}复制接入码
      </Button>
      <ol>
        <li>
          将 <b>gpu-agent</b> 放到服务器用户目录。
        </li>
        <li>运行以下命令，交互输入接入码。</li>
        <li>
          运行 <b>./gpu-agent start</b> 开始上报。
        </li>
      </ol>
      <pre>{`./gpu-agent setup --server ${location.origin}`}</pre>
      <p className="muted tiny">
        从远端接入时，请使用服务器可达的中心 IP；不要使用 localhost。接入码不放进启动参数。
      </p>
    </div>
  );
}
