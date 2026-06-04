import { Copy } from "lucide-react";
import { useState } from "react";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import type { TCredentialSecret } from "@/types/admin";

type TSecretRevealDialogProps = {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  secret: TCredentialSecret | null;
  title?: string;
};

export const SecretRevealDialog = ({
  open,
  onOpenChange,
  secret,
  title = "凭证密钥（仅此一次可见）",
}: TSecretRevealDialogProps) => {
  const [copied, setCopied] = useState(false);

  const copyAll = async () => {
    if (!secret) return;
    const text = `secret_id: ${secret.secret_id}\nsecret_key: ${secret.secret_key}`;
    await navigator.clipboard.writeText(text);
    setCopied(true);
    setTimeout(() => setCopied(false), 2000);
  };

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-w-md">
        <DialogHeader>
          <DialogTitle>{title}</DialogTitle>
          <DialogDescription className="text-amber-600 dark:text-amber-400">
            secretKey 仅此一次返回，关闭后将无法再次查看，请立即复制并妥善保存。
          </DialogDescription>
        </DialogHeader>
        {secret ? (
          <div className="space-y-4">
            <div className="space-y-2">
              <label className="text-sm font-medium">Secret ID</label>
              <Input readOnly value={secret.secret_id} />
            </div>
            <div className="space-y-2">
              <label className="text-sm font-medium">Secret Key</label>
              <Input readOnly value={secret.secret_key} className="font-mono" />
            </div>
            <Button className="w-full" onClick={() => void copyAll()}>
              <Copy className="mr-2 h-4 w-4" />
              {copied ? "已复制" : "复制全部"}
            </Button>
          </div>
        ) : null}
      </DialogContent>
    </Dialog>
  );
};
