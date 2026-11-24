import { useEffect, useRef } from 'react';
import { Link, useNavigate } from 'react-router-dom';
import { ArrowLeft } from 'lucide-react';
import { Button } from '../../components/ui/primitives';
import { HomeLogo } from '../../components/ui/HomeLogo';
import { homeRoute } from './routes';

export function NotFoundPage() {
  const heading = useRef<HTMLHeadingElement>(null);
  const navigate = useNavigate();

  useEffect(() => {
    const previousTitle = document.title;
    document.title = `404 页面未找到 · ${previousTitle}`;
    heading.current?.focus({ preventScroll: true });
    return () => { document.title = previousTitle; };
  }, []);

  function goBack() {
    const hasPreviousEntry = window.history.state?.idx > 0;
    const fromAppPage = document.referrer.startsWith(window.location.origin + '/');
    if (hasPreviousEntry || (fromAppPage && window.history.length > 1)) {
      void navigate(-1);
      return;
    }
    void navigate(homeRoute, { replace: true });
  }

  return (
    <main className="grid min-h-[100dvh] place-items-center bg-workspace px-6 py-10 font-sans text-ink sm:px-10">
      <div className="grid w-full max-w-[900px] items-center gap-7 md:grid-cols-2 md:gap-14">
        <img
          src="/logo.png"
          alt=""
          width={1254}
          height={1254}
          draggable={false}
          className="not-found-mascot w-full max-w-[200px] select-none justify-self-center md:max-w-[360px]"
        />
        <div className="text-center md:text-left">
          <h1 ref={heading} tabIndex={-1} className="outline-none">
            <span className="mb-3 block text-[clamp(3rem,7vw,5.25rem)] font-bold leading-none tracking-tight tabular-nums text-muted">404</span>
            <span className="block text-[clamp(1.875rem,4.5vw,3.5rem)] font-bold leading-tight">页面未找到</span>
          </h1>
          <div className="mt-7 flex flex-wrap items-center justify-center gap-2.5 md:justify-start">
            <Link to={homeRoute} className="ui-primary inline-flex h-10 items-center gap-2 rounded-[var(--radius-control)] px-4 text-sm font-semibold focus-visible:underline focus-visible:underline-offset-4">
              <HomeLogo />
              返回首页
            </Link>
            <Button type="button" variant="secondary" icon={ArrowLeft} className="h-10 px-4 focus-visible:underline focus-visible:underline-offset-4" onClick={goBack}>
              返回上页
            </Button>
          </div>
        </div>
      </div>
    </main>
  );
}
