// Release downloads: https://coderefinery.dev/dl/<key> serves <key> from the
// R2 bucket coderefinery-releases (binding RELEASES, see ../../wrangler.toml).
// Layout (docs/release.md): latest.json, vX.Y.Z/coderefinery_*.tar.gz,
// vX.Y.Z/SHA256SUMS, vX.Y.Z/SHA256SUMS.sig. Read-only: GET and HEAD, with
// conditional requests (ETag); no ranges.
//
// Types are declared here, not imported, so the site needs no extra package.

interface R2Object {
  key: string;
  size: number;
  httpEtag: string;
  uploaded: Date;
  writeHttpMetadata(headers: Headers): void;
}

interface R2ObjectBody extends R2Object {
  body: ReadableStream;
}

interface R2Bucket {
  get(key: string, options?: { onlyIf?: Headers }): Promise<R2ObjectBody | R2Object | null>;
  head(key: string): Promise<R2Object | null>;
}

interface Env {
  RELEASES: R2Bucket;
}

interface Context {
  request: Request;
  env: Env;
  params: { path?: string | string[] };
}

// Keys look like latest.json or v1.2.3/<file>; anything else is a 404
// without touching the bucket.
const KEY = /^(latest\.json|v\d+\.\d+\.\d+(-[0-9A-Za-z.-]+)?\/[0-9A-Za-z._-]+)$/;

function contentType(key: string): string {
  if (key.endsWith(".json")) return "application/json; charset=utf-8";
  if (key.endsWith(".tar.gz")) return "application/gzip";
  return "text/plain; charset=utf-8";
}

// latest.json moves with every release; a versioned file never changes.
function cacheControl(key: string): string {
  return key === "latest.json" ? "public, max-age=60" : "public, max-age=31536000, immutable";
}

function notFound(): Response {
  return new Response("not found\n", {
    status: 404,
    headers: { "content-type": "text/plain; charset=utf-8", "cache-control": "public, max-age=60" },
  });
}

export async function onRequest({ request, env, params }: Context): Promise<Response> {
  if (request.method !== "GET" && request.method !== "HEAD") {
    return new Response("method not allowed\n", { status: 405, headers: { allow: "GET, HEAD" } });
  }
  const key = Array.isArray(params.path) ? params.path.join("/") : params.path ?? "";
  if (!KEY.test(key)) return notFound();

  const object =
    request.method === "HEAD"
      ? await env.RELEASES.head(key)
      : await env.RELEASES.get(key, { onlyIf: request.headers });
  if (object === null) return notFound();

  const headers = new Headers();
  object.writeHttpMetadata(headers);
  headers.set("content-type", contentType(key));
  headers.set("cache-control", cacheControl(key));
  headers.set("etag", object.httpEtag);
  headers.set("last-modified", object.uploaded.toUTCString());
  headers.set("x-content-type-options", "nosniff");

  if (request.method === "HEAD") {
    headers.set("content-length", String(object.size));
    return new Response(null, { headers });
  }
  // A failed If-None-Match / If-Match precondition: get() returns the object without a body.
  if (!("body" in object)) {
    return new Response(null, { status: request.headers.has("if-none-match") ? 304 : 412, headers });
  }
  // The R2 body has a known length, so the runtime sends content-length itself.
  return new Response(object.body, { headers });
}
