import React from 'react';
import { useLocation, useNavigate } from 'react-router-dom';
import { useDeckStore, type PageView } from '../../stores/deckStore';
import { useProjectStore } from '../../stores/projectStore';
import { projectWorkspaceRoute } from './routes';

type PreviewMode = 'main' | 'overview';

function parseView(value: string | null): PageView {
  return value === 'outline' ? 'outline' : 'html';
}

function parseMode(value: string | null): PreviewMode {
  return value === 'overview' ? 'overview' : 'main';
}

export function useWorkspaceUrlState(projectId: string | undefined) {
  const location = useLocation();
  const navigate = useNavigate();
  const [hydratedProject, setHydratedProject] = React.useState<string | null>(null);
  const slides = useProjectStore((state) => projectId ? state.slidesByProjectId[projectId] ?? [] : []);
  const currentPage = useDeckStore((state) => state.currentPage);
  const globalView = useDeckStore((state) => state.globalView);
  const previewMode = useDeckStore((state) => state.previewMode);
  const setCurrentPage = useDeckStore((state) => state.setCurrentPage);
  const setGlobalView = useDeckStore((state) => state.setGlobalView);
  const enterOverview = useDeckStore((state) => state.enterOverview);
  const exitOverview = useDeckStore((state) => state.exitOverview);

  React.useEffect(() => {
    setHydratedProject(null);
  }, [projectId]);

  React.useEffect(() => {
    if (!projectId) return;
    const params = new URLSearchParams(location.search);
    const nextView = parseView(params.get('view'));
    if (nextView !== useDeckStore.getState().globalView) {
      setGlobalView(nextView);
    }

    const nextMode = parseMode(params.get('mode'));
    if (nextMode !== useDeckStore.getState().previewMode) {
      if (nextMode === 'overview') enterOverview();
      else exitOverview();
    }
  }, [enterOverview, exitOverview, location.search, projectId, setGlobalView]);

  React.useEffect(() => {
    if (!projectId || slides.length === 0) return;
    const slideId = new URLSearchParams(location.search).get('slide');
    if (slideId) {
      const index = slides.findIndex((slide) => slide.id === slideId);
      if (index >= 0 && index !== useDeckStore.getState().currentPage) {
        setCurrentPage(index);
      }
    }
    setHydratedProject(projectId);
  }, [location.search, projectId, setCurrentPage, slides]);

  React.useEffect(() => {
    if (!projectId || slides.length === 0 || hydratedProject !== projectId) return;
    const current = `${location.pathname}${location.search}`;
    const currentSlide = slides[Math.min(currentPage, slides.length - 1)];
    const target = projectWorkspaceRoute(projectId, {
      slideId: currentSlide?.id,
      view: globalView,
      mode: previewMode,
    });
    if (target !== current) {
      navigate(target, { replace: true });
    }
  }, [currentPage, globalView, hydratedProject, location.pathname, location.search, navigate, previewMode, projectId, slides]);
}
