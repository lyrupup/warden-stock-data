import { Inbox } from "lucide-react";

export const EmptyState = ({ message = "暂无数据" }: { message?: string }) => (
  <div className="flex flex-col items-center justify-center py-16 text-muted-foreground">
    <Inbox className="mb-3 h-10 w-10 opacity-50" />
    <p className="text-sm">{message}</p>
  </div>
);
