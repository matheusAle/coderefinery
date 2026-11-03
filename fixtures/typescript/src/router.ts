export type Handler = (req: Request, res: Response) => void;

export interface Request {
  params: Record<string, string>;
  query: Record<string, string | undefined>;
  body?: Record<string, unknown>;
}

export interface Response {
  statusCode: number;
  payload: unknown;
  status(code: number): Response;
  json(body: unknown): Response;
}

export class Router {
  routes: Array<{ method: string; path: string; handler: Handler }> = [];

  get(path: string, handler: Handler): void {
    this.routes.push({ method: "GET", path, handler });
  }

  post(path: string, handler: Handler): void {
    this.routes.push({ method: "POST", path, handler });
  }

  delete(path: string, handler: Handler): void {
    this.routes.push({ method: "DELETE", path, handler });
  }

  handle(method: string, path: string, req: Request, res: Response): boolean {
    const route = this.routes.find((r) => r.method === method && r.path === path);
    if (!route) {
      return false;
    }
    route.handler(req, res);
    return true;
  }
}

export function makeResponse(): Response {
  const res: Response = {
    statusCode: 200,
    payload: undefined,
    status(code: number) {
      res.statusCode = code;
      return res;
    },
    json(body: unknown) {
      res.payload = body;
      return res;
    },
  };
  return res;
}
