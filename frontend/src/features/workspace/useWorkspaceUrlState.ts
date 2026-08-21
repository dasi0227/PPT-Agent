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
  const [hydratedLocationKey, setHydratedLocationKey] = React.useState<string | null>(null);
  const slides = useProjectStore((state) => projectId ? state.slidesByProjectId[projectId] ?? [] : []);
  const contentReady = useProjectStore((state) => projectId
    ? Boolean(state.specByProjectId[projectId]) || (state.slidesByProjectId[projectId]?.length ?? 0) > 0
    : false);
  const currentSlideId = useDeckStore((state) => state.currentSlideId);
  const globalView = useDeckStore((state) => state.globalView);
  const previewMode = useDeckStore((state) => state.previewMode);
  const setCurrentSlideId = useDeckStore((state) => state.setCurrentSlideId);
  const setGlobalView = useDeckStore((state) => state.setGlobalView);
  const enterOverview = useDeckStore((state) => state.enterOverview);
  const exitOverview = useDeckStore((state) => state.exitOverview);

  React.useLayoutEffect(() => {
    if (!projectId || !contentReady) return;
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
    const requestedSlideId = params.get('slide');
    const requestedSlideExists = requestedSlideId
      ? slides.some((slide) => slide.id === requestedSlideId)
      : false;
    const nextSlideId = requestedSlideExists && requestedSlideId ? requestedSlideId : slides[0]?.id ?? null;
    if (nextSlideId !== useDeckStore.getState().currentSlideId) {
      setCurrentSlideId(nextSlideId);
    }
    setHydratedLocationKey(location.key);
  }, [contentReady, enterOverview, exitOverview, location.key, location.search, projectId, setCurrentSlideId, setGlobalView, slides]);

  React.useEffect(() => {
    if (!projectId || !contentReady || hydratedLocationKey !== location.key) return;
    const current = `${location.pathname}${location.search}`;
    const selectedSlideExists = currentSlideId
      ? slides.some((slide) => slide.id === currentSlideId)
      : false;
    const selectedSlideId = selectedSlideExists && currentSlideId ? currentSlideId : slides[0]?.id;
    const target = projectWorkspaceRoute(projectId, {
      slideId: selectedSlideId,
      view: globalView,
      mode: previewMode,
    });
    if (target !== current) {
      navigate(target, { replace: true });
    }
  }, [contentReady, currentSlideId, globalView, hydratedLocationKey, location.key, location.pathname, location.search, navigate, previewMode, projectId, slides]);
}
