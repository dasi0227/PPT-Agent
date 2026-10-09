import { AppearanceActions } from '../../components/ui/AppearanceActions';

const slideLayers = ['back', 'mid', 'front'] as const;

export const WorkspaceEmptyState = () => {
  return (
    <div className="workspace-home">
      <div className="absolute right-4 top-4 z-10"><AppearanceActions /></div>

      <div className="workspace-home-copy">
        <h1 className="workspace-home-slogan">
          <span className="workspace-home-slogan-line">
            <span className="workspace-home-agent" data-text="Agent">Agent</span>
            {' '}时代下的 <span className="workspace-home-ppt">PPT</span>
          </span>
          <span className="workspace-home-slogan-line">
            交给 <span className="workspace-home-dasi">Dasi</span> 就好了
          </span>
        </h1>
      </div>

      <div className="workspace-home-visual">
        <div className="workspace-home-slides" aria-hidden="true">
          {slideLayers.map((layer) => (
            <div key={layer} className={`workspace-home-slide workspace-home-slide-${layer}`}>
              <span className="workspace-home-slide-title" />
              <span className="workspace-home-slide-subtitle" />
              <span className="workspace-home-slide-circle" />
              <span className="workspace-home-slide-block" />
              <span className="workspace-home-chart">
                <span />
                <span />
                <span />
                <span />
              </span>
            </div>
          ))}
        </div>
        <div className="workspace-home-action-slot">
          <button
            className="workspace-home-action ui-interactive"
            type="button"
            onClick={() => {
              document.dispatchEvent(new CustomEvent('open-project-picker'));
            }}
          >
            新建 / 打开项目
          </button>
        </div>
      </div>

      <img className="workspace-home-logo" src="/logo.png" alt="Dasi" width={1280} height={1254} draggable={false} />
    </div>
  );
};
