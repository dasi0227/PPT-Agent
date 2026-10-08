import React from 'react';
import { useLocation, useNavigate } from 'react-router-dom';
import { useDeckStore, type PageView } from '../../stores/deckStore';
import { useProjectStore } from '../../stores/projectStore';
import { projectWorkspaceRoute } from './routes';
import { orderedSlides } from '../deck/selectors';

type PreviewMode = 'main' | 'overview';

function parseView(value: string | null): PageView {
  return value === 'spec' ? 'outline' : 'html';
}

function parseMode(value: string | null): PreviewMode {
  return value === 'overview' ? 'overview' : 'main';
}

export function useWorkspaceUrlState(projectId: string | undefined) {
  const location = useLocation();
  const navigate = useNavigate();
  const [hydratedLocationKey, setHydratedLocationKey] = React.useState<string | null>(null);
  const snapshot = useProjectStore((state) => projectId ? state.contentByProjectId[projectId] : undefined);
  const slides = React.useMemo(() => orderedSlides(snapshot), [snapshot]);
  const previousSlides = React.useRef<{ projectId?: string; ids: string[] }>({ ids: [] });
  const contentReady = Boolean(snapshot);
  const currentSlideId = useDeckStore((state) => state.currentSlideId);
  const activeDocument = useDeckStore((state) => state.activeDocument);
  const globalView = useDeckStore((state) => state.globalView);
  const previewMode = useDeckStore((state) => state.previewMode);
  const contentMode = useDeckStore((state) => state.contentMode);
  React.useLayoutEffect(() => {
    if (!projectId || !contentReady || hydratedLocationKey === location.key) return;
    const params = new URLSearchParams(location.search);
    const document = params.get('document');
    const nextDocument = document === 'manifest' || document === 'design' || document === 'outline' ? document : null;
    const nextMode = nextDocument ? 'main' : parseMode(params.get('mode'));
    const nextView = nextDocument || nextMode === 'overview' ? 'html' : parseView(params.get('view'));
    const nextContent = nextMode === 'main' && nextDocument !== 'outline' && params.get('content') === 'source' ? 'source' : 'preview';
    const requestedSlideId = params.get('slide');
    const rememberedSlideId = useDeckStore.getState().currentSlideId;
    const selectedSlideId = nextDocument || nextMode === 'overview' ? rememberedSlideId : requestedSlideId;
    const nextSlideId = slides.some(slide => slide.id === selectedSlideId) ? selectedSlideId : slides[0]?.id ?? null;
    useDeckStore.setState({
      currentSlideId: nextSlideId,
      activeDocument: nextDocument,
      globalView: nextView,
      previewMode: nextMode,
      contentMode: nextContent,
    });
    setHydratedLocationKey(location.key);
  }, [contentReady, hydratedLocationKey, location.key, location.search, projectId, slides]);

  React.useLayoutEffect(() => {
    if (!projectId || !contentReady || hydratedLocationKey !== location.key) return;
    const current = `${location.pathname}${location.search}`;
    const selectedSlideExists = currentSlideId
      ? slides.some((slide) => slide.id === currentSlideId)
      : false;
    const oldIndex = previousSlides.current.projectId === projectId
      ? previousSlides.current.ids.indexOf(currentSlideId ?? '')
      : -1;
    const selectedSlideId = selectedSlideExists && currentSlideId
      ? currentSlideId
      : slides[Math.min(Math.max(oldIndex, 0), slides.length - 1)]?.id;
    previousSlides.current = { projectId, ids: slides.map((slide) => slide.id) };
    if (!selectedSlideExists && currentSlideId !== (selectedSlideId ?? null)) {
      // Background page updates must not navigate away from a project document.
      useDeckStore.setState({ currentSlideId: selectedSlideId ?? null });
    }
    const target = projectWorkspaceRoute(projectId, {
      slideId: selectedSlideId,
      view: globalView,
      mode: previewMode,
      content: contentMode,
      document: activeDocument,
    });
    if (target !== current) {
      navigate(target, { replace: true });
    }
  }, [activeDocument, contentMode, contentReady, currentSlideId, globalView, hydratedLocationKey, location.key, location.pathname, location.search, navigate, previewMode, projectId, slides]);
}
