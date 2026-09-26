import { formatHTMLSource } from './htmlSourceFormatter';

interface FormatRequest { id: number; source: string }
interface FormatResponse { id: number; content: string }

const workerScope = self as unknown as {
  onmessage: ((event: MessageEvent<FormatRequest>) => void) | null;
  postMessage: (message: FormatResponse) => void;
};

workerScope.onmessage = (event) => {
  const { id, source } = event.data;
  void formatHTMLSource(source)
    .then((content) => workerScope.postMessage({ id, content }))
    .catch(() => workerScope.postMessage({ id, content: source }));
};
