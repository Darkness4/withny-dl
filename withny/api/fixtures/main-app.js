try {
    let _ = "undefined" != typeof window ? window : "undefined" != typeof global ? global : "undefined" != typeof globalThis ? globalThis : "undefined" != typeof self ? self : {}
      , e = (new _.Error).stack;
    e && (_._sentryDebugIds = _._sentryDebugIds || {},
    _._sentryDebugIds[e] = "ec6911f6-7309-498d-b760-803a5853aea3",
    _._sentryDebugIdIdentifier = "sentry-dbid-ec6911f6-7309-498d-b760-803a5853aea3")
} catch (_) {}
(self.webpackChunk_N_E = self.webpackChunk_N_E || []).push([[7358], {
    17629: (_, e, E) => {
        Promise.resolve().then(E.t.bind(E, 39065, 23)),
        Promise.resolve().then(E.t.bind(E, 33283, 23)),
        Promise.resolve().then(E.t.bind(E, 69699, 23)),
        Promise.resolve().then(E.t.bind(E, 34712, 23)),
        Promise.resolve().then(E.t.bind(E, 47132, 23)),
        Promise.resolve().then(E.t.bind(E, 87748, 23)),
        Promise.resolve().then(E.t.bind(E, 50700, 23)),
        Promise.resolve().then(E.t.bind(E, 75082, 23))
    }
    ,
    27941: (_, e, E) => {
        "use strict";
        var I = E(34248)
          , P = E(58989)
          , N = E(89173);
        "production" === I._.NODE_ENV && P.Ts({
            dsn: I._.NEXT_PUBLIC_SENTRY_DSN,
            tracesSampleRate: 1,
            debug: !1
        }),
        N.Nc
    }
    ,
    34248: (_, e, E) => {
        "use strict";
        E.d(e, {
            _: () => T
        });
        var I = E(34983)
          , P = E(43343)
          , N = E(40459);
        let T = (0,
        I.w)({
            shared: {
                NODE_ENV: P.KCZ([P.euz("production"), P.euz("development"), P.euz("test")]),
                ANALYZE: P.lqM(P.YjP())
            },
            client: {
                NEXT_PUBLIC_APP_ROOT: P.FsL(P.YjP(), P.OZ5()),
                NEXT_PUBLIC_API_ROOT: P.FsL(P.YjP(), P.OZ5()),
                NEXT_PUBLIC_TELECOM_CLIENTIP: P.FsL(P.YjP(), P.pdi(_ => Number(_))),
                NEXT_PUBLIC_CREDIX_CLIENTIP: P.FsL(P.YjP(), P.pdi(_ => Number(_))),
                NEXT_PUBLIC_PAIDY_API_KEY: P.YjP(),
                NEXT_PUBLIC_GA4_ID: P.YjP(),
                NEXT_PUBLIC_GTM_ID: P.YjP(),
                NEXT_PUBLIC_SENTRY_DSN: P.FsL(P.YjP(), P.OZ5()),
                NEXT_PUBLIC_MICROCMS_SERVICE_DOMAIN: P.YjP(),
                NEXT_PUBLIC_MICROCMS_API_KEY: P.YjP(),
                NEXT_PUBLIC_GOOGLE_CLIENT_ID: P.YjP(),
                NEXT_PUBLIC_GOOGLE_REDIRECT_URI: P.FsL(P.YjP(), P.OZ5()),
                NEXT_PUBLIC_GOOGLE_OAUTH_STATE: P.YjP(),
                NEXT_PUBLIC_CF_TURNSTILE_SITE_KEY: P.YjP(),
                NEXT_PUBLIC_GRAPHQL_ENDPOINT: P.FsL(P.YjP(), P.OZ5()),
                NEXT_PUBLIC_GRAPHQL_FAKE_TOKEN: P.YjP(),
                NEXT_PUBLIC_CAST_APPLY_GAS_URL: P.FsL(P.YjP(), P.OZ5())
            },
            server: {
                NEXT_RUNTIME: P.lqM(P.KCZ([P.euz("nodejs"), P.euz("edge")]), "nodejs"),
                BANKCODEJP_API_KEY: P.YjP()
            },
            runtimeEnv: {
                NODE_ENV: "production",
                ANALYZE: N.env.ANALYZE,
                NEXT_PUBLIC_APP_ROOT: "https://www.withny.fun",
                NEXT_PUBLIC_API_ROOT: "https://api.withny.fun",
                NEXT_PUBLIC_TELECOM_CLIENTIP: "40435",
                NEXT_PUBLIC_CREDIX_CLIENTIP: "1011004407",
                NEXT_PUBLIC_PAIDY_API_KEY: "pk_live_uibr9tbr3nhauodciaj2ppva2k",
                NEXT_PUBLIC_GA4_ID: "G-1N6VTZ6D0K",
                NEXT_PUBLIC_GTM_ID: "GTM-K4WB4VG",
                NEXT_RUNTIME: "",
                NEXT_PUBLIC_SENTRY_DSN: "https://725f5b0bf81b27af0b8d996464832b30@o1323119.ingest.us.sentry.io/4507193236652032",
                NEXT_PUBLIC_MICROCMS_SERVICE_DOMAIN: "withny",
                NEXT_PUBLIC_MICROCMS_API_KEY: "A1P4Ur7eRGvwuBjYyboIUjJ7hx5wtJWZ5x7Q",
                NEXT_PUBLIC_GOOGLE_CLIENT_ID: "663505044448-9kl291fstl7s0ldforqk3c2s223m6ujd.apps.googleusercontent.com",
                NEXT_PUBLIC_GOOGLE_REDIRECT_URI: "https://www.withny.fun/oauth/google/callback",
                NEXT_PUBLIC_GOOGLE_OAUTH_STATE: "r2iSfPXNHBFGbqr8WWjgWydJy4NrW5WfnAkkU9Tq4A=",
                NEXT_PUBLIC_CF_TURNSTILE_SITE_KEY: "0x4AAAAAAA_6TeBp4BVaRAvY",
                NEXT_PUBLIC_GRAPHQL_ENDPOINT: "https://77fkxz2qsvclbkkbvzxjt2jley.appsync-api.ap-northeast-1.amazonaws.com/graphql",
                NEXT_PUBLIC_GRAPHQL_FAKE_TOKEN: "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJ1dWlkIjoiMGRkMDJmMGMtZDA4MC00YjE2LWE0NTItNmZhNTVmNjZjZTAyIiwic2NvcGUiOiJ1c2VyIiwiaWF0IjoxNzAyNzM2NTA2LCJleHAiOjE3MDI4MjI5MDZ9.D61KDoNNxNXNJsddXsvvh_x7ztKFIgr9UVxnlRbiz7U",
                NEXT_PUBLIC_CAST_APPLY_GAS_URL: "https://script.google.com/macros/s/AKfycbxo2jWKCy3ntZl5jZNjz2OJmU6aX6MQ1PrMtxqqvAwxnTqlwZvZJK8FU6-kgVHBYw_M/exec",
                BANKCODEJP_API_KEY: N.env.BANKCODEJP_API_KEY
            }
        })
    }
}, _ => {
    var e = e => _(_.s = e);
    _.O(0, [9029, 587, 3477], () => (e(27941),
    e(45504),
    e(17629))),
    _N_E = _.O()
}
]);
