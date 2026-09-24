import React from 'react';
import { useLocation, useNavigate } from 'react-router-dom';
import { useDeckStore, type PageView } from '../../stores/deckStore';
import { useProjectStore } from '../../stores/projectStore';
import { projectWorkspaceRoute } from './routes';
import { orderedSlides } from '../deck/selectors';

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
  const snapshot = useProjectStore((state) => projectId ? state.contentByProjectId[projectId] : undefined);
  const slides = React.useMemo(() => orderedSlides(snapshot), [snapshot]);
  const previousSlides = React.useRef<{ projectId?: string; ids: string[] }>({ ids: [] });
  const contentReady = Boolean(snapshot);
  const currentSlideId = useDeckStore((state) => state.currentSlideId);
  const activeDocument = useDeckStore((state) => state.activeDocument);
  const globalView = useDeckStore((state) => state.globalView);
  const previewMode = useDeckStore((state) => state.previewMode);
  const contentMode = useDeckStore((state) => state.contentMode);
  const setContentMode = useDeckStore((state) => state.setContentMode);
  const setCurrentSlideId = useDeckStore((state) => state.setCurrentSlideId);
  const setActiveDocument = useDeckStore((state) => state.setActiveDocument);
  const setGlobalView = useDeckStore((state) => state.setGlobalView);
  const enterOverview = useDeckStore((state) => state.enterOverview);
  const exitOverview = useDeckStore((state) => state.exitOverview);

  React.useLayoutEffect(() => {
    if (!projectId || !contentReady || hydratedLocationKey === location.key) return;
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
    const remembered = sessionStorage.getItem(`ppt-agent-content-mode-${projectId}`);
    const nextContent = params.get('content') === 'source' || (!params.has('content') && remembered === 'source') ? 'source' : 'preview';
    if (nextContent !== useDeckStore.getState().contentMode) setContentMode(nextContent);
    const requestedSlideId = params.get('slide');
    const requestedSlideExists = requestedSlideId
      ? slides.some((slide) => slide.id === requestedSlideId)
      : false;
    const nextSlideId = requestedSlideExists && requestedSlideId ? requestedSlideId : slides[0]?.id ?? null;
    if (nextSlideId !== useDeckStore.getState().currentSlideId) {
      setCurrentSlideId(nextSlideId);
    }
    const document = params.get('document');
    const nextDocument = document === 'manifest' || document === 'design' ? document : null;
    if (nextDocument !== useDeckStore.getState().activeDocument) {
      // Apply after page state: selecting a page clears the document selection.
      if (nextDocument) setActiveDocument(nextDocument);
      else useDeckStore.setState({ activeDocument: null });
    }
    setHydratedLocationKey(location.key);
  }, [contentReady, enterOverview, exitOverview, hydratedLocationKey, location.key, location.search, projectId, setActiveDocument, setContentMode, setCurrentSlideId, setGlobalView, slides]);

  React.useEffect(() => { if (projectId && hydratedLocationKey === location.key) sessionStorage.setItem(`ppt-agent-content-mode-${projectId}`, contentMode); }, [contentMode, hydratedLocationKey, location.key, projectId]);

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
