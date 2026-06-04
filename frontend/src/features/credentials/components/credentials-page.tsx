import { Copy, Plus, RefreshCw } from "lucide-react";
import { useState } from "react";
import { Link } from "react-router-dom";
import { ConfirmDialog } from "@/components/common/confirm-dialog";
import { PageHeader } from "@/components/common/page-header";
import { SecretRevealDialog } from "@/components/common/secret-reveal-dialog";
import { EmptyState } from "@/components/common/empty-state";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { usePagedQuery } from "@/hooks/use-paged-query";
import { maskSecretId } from "@/lib/format";
import type { TCredentialSecret } from "@/types/admin";
import {
  useCreateCredential,
  useCredentials,
  useRevokeCredential,
  useRotateCredential,
} from "../hooks/use-credentials";

export const CredentialsPage = () => {
  const { page, size, setPage } = usePagedQuery();
  const { data, isLoading } = useCredentials(page, size);
  const createMutation = useCreateCredential();
  const rotateMutation = useRotateCredential();
  const revokeMutation = useRevokeCredential();

  const [createOpen, setCreateOpen] = useState(false);
  const [consumerName, setConsumerName] = useState("");
  const [secret, setSecret] = useState<TCredentialSecret | null>(null);
  const [secretOpen, setSecretOpen] = useState(false);
  const [revokeId, setRevokeId] = useState<number | null>(null);

  const handleCreate = async () => {
    const result = await createMutation.mutateAsync({
      consumer_name: consumerName,
      rate_limit: 20,
      daily_quota: 100000,
    });
    setCreateOpen(false);
    setConsumerName("");
    setSecret(result);
    setSecretOpen(true);
  };

  const handleRotate = async (id: number) => {
    const result = await rotateMutation.mutateAsync(id);
    setSecret(result);
    setSecretOpen(true);
  };

  const copySecretId = async (secretId: string) => {
    await navigator.clipboard.writeText(secretId);
  };

  const totalPages = data ? Math.ceil(data.total / size) : 1;

  return (
    <>
      <PageHeader
        title="凭证管理"
        description="为接入方分发 secretId / secretKey，接入方仅具只读权限"
        actions={
          <Button onClick={() => setCreateOpen(true)}>
            <Plus className="mr-2 h-4 w-4" />
            创建凭证
          </Button>
        }
      />

      {isLoading ? (
        <p className="text-muted-foreground">加载中…</p>
      ) : !data?.list.length ? (
        <EmptyState message="暂无凭证，点击创建" />
      ) : (
        <>
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>接入方</TableHead>
                <TableHead>Secret ID</TableHead>
                <TableHead>Scope</TableHead>
                <TableHead>限流 QPS</TableHead>
                <TableHead>日配额</TableHead>
                <TableHead>状态</TableHead>
                <TableHead className="text-right">操作</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {data.list.map((c) => (
                <TableRow key={c.id}>
                  <TableCell>
                    <Link
                      to={`/credentials/${c.id}`}
                      className="font-medium text-primary hover:underline"
                    >
                      {c.consumer_name}
                    </Link>
                  </TableCell>
                  <TableCell>
                    <span className="font-mono text-xs">{maskSecretId(c.secret_id)}</span>
                    <Button
                      variant="ghost"
                      size="icon"
                      className="ml-1 h-7 w-7"
                      onClick={() => void copySecretId(c.secret_id)}
                    >
                      <Copy className="h-3 w-3" />
                    </Button>
                  </TableCell>
                  <TableCell>{c.scope}</TableCell>
                  <TableCell>{c.rate_limit}</TableCell>
                  <TableCell>{c.daily_quota.toLocaleString()}</TableCell>
                  <TableCell>
                    <Badge variant={c.status === 1 ? "success" : "secondary"}>
                      {c.status === 1 ? "启用" : "停用"}
                    </Badge>
                  </TableCell>
                  <TableCell className="text-right space-x-1">
                    <Button
                      variant="outline"
                      size="sm"
                      disabled={rotateMutation.isPending}
                      onClick={() => void handleRotate(c.id)}
                    >
                      <RefreshCw className="mr-1 h-3 w-3" />
                      轮换
                    </Button>
                    <Button
                      variant="destructive"
                      size="sm"
                      onClick={() => setRevokeId(c.id)}
                    >
                      吊销
                    </Button>
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
          <div className="mt-4 flex items-center justify-between">
            <span className="text-sm text-muted-foreground">共 {data.total} 条</span>
            <div className="flex gap-2">
              <Button
                variant="outline"
                size="sm"
                disabled={page <= 1}
                onClick={() => setPage(page - 1)}
              >
                上一页
              </Button>
              <span className="flex items-center text-sm">
                {page} / {totalPages}
              </span>
              <Button
                variant="outline"
                size="sm"
                disabled={page >= totalPages}
                onClick={() => setPage(page + 1)}
              >
                下一页
              </Button>
            </div>
          </div>
        </>
      )}

      <Dialog open={createOpen} onOpenChange={setCreateOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>创建接入凭证</DialogTitle>
          </DialogHeader>
          <div className="space-y-4">
            <div className="space-y-2">
              <Label>接入方名称</Label>
              <Input
                value={consumerName}
                onChange={(e) => setConsumerName(e.target.value)}
                placeholder="warden-stock-trading"
              />
            </div>
            <Button
              className="w-full"
              disabled={!consumerName || createMutation.isPending}
              onClick={() => void handleCreate()}
            >
              创建
            </Button>
          </div>
        </DialogContent>
      </Dialog>

      <SecretRevealDialog
        open={secretOpen}
        onOpenChange={setSecretOpen}
        secret={secret}
      />

      <ConfirmDialog
        open={revokeId !== null}
        onOpenChange={(o) => !o && setRevokeId(null)}
        title="吊销凭证"
        description="吊销后该凭证将立即失效，接入方将无法继续调用 API。此操作不可撤销。"
        confirmLabel="确认吊销"
        loading={revokeMutation.isPending}
        onConfirm={() => {
          if (revokeId) {
            void revokeMutation.mutateAsync(revokeId).then(() => setRevokeId(null));
          }
        }}
      />
    </>
  );
};
