// Redirect www.coderefinery.dev → coderefinery.dev (same path and query).
// Done here rather than in public/_redirects because Pages applies the
// static host rule inconsistently across custom domains.
export const onRequest: PagesFunction = async ({ request, next }) => {
  const url = new URL(request.url);
  if (url.hostname === "www.coderefinery.dev") {
    url.hostname = "coderefinery.dev";
    return Response.redirect(url.toString(), 301);
  }
  return next();
};
