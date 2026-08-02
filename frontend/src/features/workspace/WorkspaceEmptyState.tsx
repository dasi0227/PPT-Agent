export const WorkspaceEmptyState = () => {
  return (
    <div className="flex h-full w-full flex-1 flex-col items-center justify-center bg-background text-text-900">
      <h1 className="mb-4 max-w-[94vw] text-center text-[clamp(3rem,8vw,8rem)] font-black italic leading-none tracking-[-0.07em] text-text-900">
        Dasi PPT Agent
      </h1>
      <div className="mb-10 flex items-center gap-4 text-text-400">
        <span aria-hidden="true" className="h-px w-16 bg-border sm:w-28" />
        <p className="whitespace-nowrap text-base font-medium">AI 时代下的 PPT 交给 Agent 就好了</p>
        <span aria-hidden="true" className="h-px w-16 bg-border sm:w-28" />
      </div>
      <button 
        className="h-10 rounded-md bg-accent px-5 text-base font-semibold text-white hover:bg-accent/90"
        onClick={() => {
          document.dispatchEvent(new CustomEvent('open-project-picker'));
        }}
      >
        + 新建 / 打开项目
      </button>
    </div>
  );
};
