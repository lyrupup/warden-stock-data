import { useParams } from "react-router-dom";
import { PageHeader } from "@/components/common/page-header";
import { EmptyState } from "@/components/common/empty-state";
import { Badge } from "@/components/ui/badge";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { formatDate, formatDateTime } from "@/lib/format";
import { useCredentialDetail } from "../hooks/use-credentials";

const HMAC_EXAMPLE = `// HMAC 签名接入示例
const stringToSign = [
  method,
  path,
  canonicalQuery,
  secretId,
  timestamp,
  nonce,
  sha256Hex(body),
].join("\\n");

const signature = Base64(HMAC_SHA256(secretKey, stringToSign));

// 请求头
X-Secret-Id: <secretId>
X-Timestamp: <unix_ms>
X-Nonce: <random>
X-Signature: <signature>`;

export const CredentialDetailPage = () => {
  const { id } = useParams<{ id: string }>();
  const credentialId = Number(id);
  const { data, isLoading } = useCredentialDetail(credentialId);

  if (isLoading) return <p className="text-muted-foreground">加载中…</p>;
  if (!data) return <EmptyState message="凭证不存在" />;

  return (
    <>
      <PageHeader
        title={data.consumer_name}
        description={`Secret ID: ${data.secret_id}`}
      />

      <div className="mb-6 grid gap-4 md:grid-cols-4">
        <Card>
          <CardHeader className="pb-2">
            <CardTitle className="text-sm font-medium text-muted-foreground">Scope</CardTitle>
          </CardHeader>
          <CardContent>
            <Badge>{data.scope}</Badge>
          </CardContent>
        </Card>
        <Card>
          <CardHeader className="pb-2">
            <CardTitle className="text-sm font-medium text-muted-foreground">限流 QPS</CardTitle>
          </CardHeader>
          <CardContent className="text-2xl font-bold">{data.rate_limit}</CardContent>
        </Card>
        <Card>
          <CardHeader className="pb-2">
            <CardTitle className="text-sm font-medium text-muted-foreground">日配额</CardTitle>
          </CardHeader>
          <CardContent className="text-2xl font-bold">
            {data.daily_quota.toLocaleString()}
          </CardContent>
        </Card>
        <Card>
          <CardHeader className="pb-2">
            <CardTitle className="text-sm font-medium text-muted-foreground">状态</CardTitle>
          </CardHeader>
          <CardContent>
            <Badge variant={data.status === 1 ? "success" : "secondary"}>
              {data.status === 1 ? "启用" : "停用"}
            </Badge>
          </CardContent>
        </Card>
      </div>

      <Card className="mb-6">
        <CardHeader>
          <CardTitle className="text-lg">调用审计（按日）</CardTitle>
        </CardHeader>
        <CardContent>
          {!data.access_stats?.length ? (
            <EmptyState message="暂无调用记录" />
          ) : (
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>日期</TableHead>
                  <TableHead>调用次数</TableHead>
                  <TableHead>错误次数</TableHead>
                  <TableHead>最近调用</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {data.access_stats.map((s) => (
                  <TableRow key={s.stat_date}>
                    <TableCell>{formatDate(s.stat_date)}</TableCell>
                    <TableCell>{s.call_count}</TableCell>
                    <TableCell>{s.error_count}</TableCell>
                    <TableCell>{formatDateTime(s.last_access_at)}</TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          )}
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle className="text-lg">HMAC 签名接入指引</CardTitle>
        </CardHeader>
        <CardContent>
          <pre className="overflow-x-auto rounded-md bg-muted p-4 text-xs leading-relaxed">
            {HMAC_EXAMPLE}
          </pre>
        </CardContent>
      </Card>
    </>
  );
};
