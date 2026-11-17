import { Link } from 'react-router-dom';

export default function NotFoundPage() {
  return (
    <section className="flex min-h-[50vh] flex-col items-center justify-center gap-3 text-center">
      <p className="text-xs font-medium uppercase tracking-wide text-indigo-600">
        404
      </p>
      <h1 className="text-2xl font-semibold tracking-tight text-slate-900">
        页面不存在
      </h1>
      <p className="max-w-md text-sm text-slate-500">
        你访问的路径不在 PatchTrace Market 的模块路由中。
      </p>
      <Link
        to="/marketplace"
        className="text-sm font-medium text-indigo-600 hover:text-indigo-500"
      >
        返回任务大厅
      </Link>
    </section>
  );
}
