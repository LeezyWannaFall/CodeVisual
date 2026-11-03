import ELK from 'elkjs/lib/elk-api'
import ElkWorker from 'elkjs/lib/elk-worker.min.js?worker'

// Vite emits the worker as a local embedded asset; no CDN or remote service.
export function createLayoutEngine() {
 return new ELK({ workerFactory: () => new ElkWorker() })
}
