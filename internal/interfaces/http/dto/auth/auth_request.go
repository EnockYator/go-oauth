package auth

// There are no request bodies in this module: /auth/login and
// /auth/callback are path-and-query driven, and /auth/logout and /auth/me
// rely exclusively on the session cookie.
//
// This file intentionally contains only the query-parameter shapes used by
// the handler, so that future additions have an obvious home.
