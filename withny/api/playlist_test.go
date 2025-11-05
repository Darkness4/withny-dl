package api_test

import (
	"strings"
	"testing"

	"github.com/Darkness4/withny-dl/withny/api"
	"github.com/stretchr/testify/require"
)

const fixture = `#EXTM3U
#EXT-X-SESSION-DATA:DATA-ID="BLABLA",VALUE="BLABLA"
#EXT-X-MEDIA:TYPE=VIDEO,GROUP-ID="720p60",NAME="720p60",AUTOSELECT=YES,DEFAULT=YES
#EXT-X-STREAM-INF:BANDWIDTH=3002999,RESOLUTION=1280x720,CODECS="avc1.4D401F,mp4a.40.2",VIDEO="720p60",FRAME-RATE=60.000
https://video-weaver.cdg02.hls.live-video.net/v1/playlist/Ct0FvodoSVcz8-D3EVrtl47CUOow1G47HAi2dqZov7sZznIr9Fzg4MmdDXqjLcaF0wKiBsk3NIEqu6uCChB58-EzEUrOXHMbUV1M-BllBjZW74D29y-8bbBBn8a_10QJW1ECfJ2pa0dR3GDiHIhlh952bzfEjsqElxqhaKMY37JqOKH3K-_jwtEWHiK18DOYm4pU59W-hzyEPk2TbuLjk6FSWn2vbBU4r1v7A_Zomk58UGuxxj4kF9YoWtvMwb7S4mn2Xj8aMJsZ-asHrPiV_kTksmDtPfRz9CRw-XNRFEjj1vn8NQLBgZVFSKhlpG5jI9U_FKMjf2mpqnoQIZC2kNUfpAi7zHPuJD0NAQ5JLS5O1448HfVpc_STYe81NpxEA75L3AHyO3xZBAt7pUTZXI7hjxzt8FMDb74ee0andg51FlRZ0zN4wXdS7c9ie4JX9zY8OQS1Mo9cfdaNfauqn7N-ZsgJaPI1W32gfI0lAkEaVg3MmOJxasQWy-7MRdsf7kKglMIMNxsNyPAIujkzy7jmbBM8orRS7U6uGlt_hxfQIg2523SaWMLXs2co9dlBxCtoWk_JZW5neXJhSrMffgK0oQBUEIuSwpsVNtPPBZmIbmIMtEYZKc18Cp2s65UqzTA1uJoaw1zONchKDpE0NDZ7wNl2h4x6ZKyD1DI-QID4mq_VnrCcu40DzzlE8YDv6vd36rdra5tFIxox0JvX-d-m-kAfljyDlOiAfgH-2J2MuPZP43o9Z6WT1vW4SBb8TZtZ-Mp8X_lj-x0vLFYz-p3Y2__qHkO-nUt-JRwULGq8D-yy1Go6Hln6FYvSaLL2EiiOgtL7bdoIlZf63mR8dM-QxWKVnuCiKwZzR1hGD57Jwbovsk-wrgp54gXJQGz_PBkiF4qvQOgeDaotH_WRlvU42XnJ1oXdvZ8VT0cVg195jPNZfb6OsjHGfjkvbFg41uwHddg4XCsy3bizH3cyCxoMxnlED5WqSf17aburIAEqCWV1LXdlc3QtMTCNBg.m3u8
#EXT-X-MEDIA:TYPE=VIDEO,GROUP-ID="480p30",NAME="480p",AUTOSELECT=YES,DEFAULT=YES
#EXT-X-STREAM-INF:BANDWIDTH=1323000,RESOLUTION=852x480,CODECS="avc1.4D401F,mp4a.40.2",VIDEO="480p30",FRAME-RATE=30.000
https://video-weaver.cdg02.hls.live-video.net/v1/playlist/CtUFJ2BaKoMlM3d1K1N9TigpH4iPSfhX2Mdv1j36wZ88zin-F2vlCN3UierFCNvpnwIXh0eiOmqEfNEpCJBto-a_NpxxcywKJW9SXRGJBU_sB4JrihnED_vxu6-w1mdHvxfEXL6D630VZIGzZFLh0Fw-rF71bm_WehZ1KD5NAXqa-gM2AUu1JUULA760wPZp3hr96vXSHJT5lNBQdNDL2PFPHA0cUExXFcK-Tw_c7vXEANClsbIRW5m4lrY3XbYfKVhYksK3QNYVdLmNEs1HzSR-ijOGDWLNDDm8HnNlC2tiEnXVxK0Ta49gzIz-TxfqRfTYbSAo8rYmMqNvpSbWd2sAUAZ458sEuUNht3L5AksJ0iGGpzhCqriQeX7JexbOyi3P8uJowUMHOv1A6PGAd4Ca_aR7I_3SiA7RakdbLFmRhXD4siKKidSRKs7JKEODpB7QKDXar9BVyoC3gTQWOqvuRL3vYXoTgGYeQOtDXSGZrNCxwJTqt2mvfCu5TvkV3dwprqSOaq8fDoLw88y3Q2hB810kSLD8L-8p3s6w-KQdyfxdpTVcwyG2hyDGd-OYVGtAXE9xp5krldEiCauCMj-WHXapQk6ooFPm0j6W3i-eLVsQZnk4uD1xPmAaXGgWPGBa2iPmOjo1VMugk-ct2PApbWB0FqhnyNRmiz6gCh_ZROKNquOFouRdDELRC0-V8GG4kjZbUhihExe4edTWSjJRAxdalkpF3dBcSqjmAElc-_6TDnM8BgebJJWrpJ4uMP_IrcKlfeT7DZgW-wc5XZlFLPGbVqQc6iaIEau8Fo1roKZF5p4hZobL2C2SxJ17FtQeUe6t1VPA3zRuDFH55aJ90gH7B4OV_uDh5NuO3l0YMT6H_oqJqzeMhe9EFIB4oUZlvmpTmIx87Tx1qRqfZl7RBWu4xnQB_g1Dumd50mSCtFAIjD19ff0PY6JN21NnySZ1zrek_w8aDAY6QqcC129usrQk_CABKglldS13ZXN0LTEwjQY.m3u8
#EXT-X-MEDIA:TYPE=VIDEO,GROUP-ID="360p30",NAME="360p",AUTOSELECT=YES,DEFAULT=YES
#EXT-X-STREAM-INF:BANDWIDTH=700000,RESOLUTION=640x360,CODECS="avc1.4D401F,mp4a.40.2",VIDEO="360p30",FRAME-RATE=30.000
https://video-weaver.cdg02.hls.live-video.net/v1/playlist/Cs0FZ4wB9jHcVfk5wj7B5shAT5_6ymmZDB7UxM4W_C9qVfYw8wl19DpTReCV5vesuHqjew9UJRMP7pE7F0rvcJbFFFWrrAMNB3lSPPddJwTZQ5HDAkN7dtpQ-cb2qVtSpoejczA7FBnAwYZVG65kht86v6r9T-gJApnasvWBEtL62bsv1RmFJMqo0m3BXnRarwNjwUSV6j5AcVPLvJljkAtDZS6KmUqBi_on0oSfnhX4koLfhlfGrwUP-qFGD2HQJ0Oh_5QNp_U1o8R19fXrs0vVBaNdLPAxWRrJcioay5LaIVANcndDgiMN3GaRBpW5ewOESwOjqYzhR-rwgLgN2zHERJ6hot5__TPwbE6e9SU58qTpTazAYkJ2M2v2r5hx1AjGvq_SVe9Qg898ySYpud8KZU9_L0Ucphfyc12mnZPphCEosQnSTXisWd07E2GfWwZ_wM3OQPw3WdUAbySou4PKSP_nfKNtEuhYfo85ojioRFEDIuMkguHWl36gBfz5lL2UoMZnpXl-Lz5RqPlh9HVehnSRdYyKLZhYs_JQlwBdgYbzNYPZbXB26aadGjZuHnloh6BU4Bj7-2pfPy0HSvlq3VmfGOJuEYF_QGiUDSomxP6LlFwJK2g9By-Z2BWJMUcC_gcqVbatyRrHXTp9TxzRGVnrfYRmodvxDdsGvPHn8STygPU7YleTRNzjXaWV3Dq9fWFvUxsnqJWkXzXRaF2LzgFtNuAQCxERP0Ac33x77zS3Yv_Ap4BbEc52ukutsRzWFXrf9BAYAv784-DrF2wRcSN9_SaQq3wMdpj8_BN_jbOII_695ujdeh50zVX8fILQUoa3eg4b94NX2n0NOsI6O4pxDI8S94CmEr7obeDY_4CHpnTXZJ8Hd0gr_HooulAayXbXYsJ2yenMStBlYXrJICoXjSxToEjqTztb-RLfdGqNPdypxNP5QsjswmS1GgynFCwYgf1ewjnJ5ZMgASoJZXUtd2VzdC0xMI0G.m3u8
#EXT-X-MEDIA:TYPE=VIDEO,GROUP-ID="160p30",NAME="160p",AUTOSELECT=YES,DEFAULT=YES
#EXT-X-STREAM-INF:BANDWIDTH=270000,RESOLUTION=284x160,CODECS="avc1.4D401F,mp4a.40.2",VIDEO="160p30",FRAME-RATE=30.000
https://video-weaver.cdg02.hls.live-video.net/v1/playlist/CsUFpHO_XIJIWpJLbecYWOV5f2Avslr3vTsurZoF6gXamlmTORcHoZfhyMTn96f8pTxstX3g9-shpgHIiutUy4zzZZYuIgRpkXqmksJnJh6sM_vO8khf40HK8SGbfyOT-pakE4xJAmcD-GgaXML7-haEsNQs02nsN0R0F9IPvZcBeKKq-86ihtZczCiF-oFKF_RyZe8Ta6NosWi0bXu-ZRWbNr3o3F7DjBMDsx9Wq9EbrNZ-489X2Ey_v6qMmrM8cHf5B_feVhzBz6TVOplANf8lYxSZxHBBH43yResARsKa9ijuapaYLRZ8R9d5mhf3gfNxdOfOcMXNIYY4Vyda7nZFTGdFouXyp0IxgB2nfGgs0Nkgzus6i3VJUd4aCyrPexhkj1GB9NWa0tIpvHDzDW5i0Po0nU-3l_PSGzOgzuKv8zME43w_ibsm9PHpuqbsiNOFz2z6Uhh80ldaSq_9tnQXO1UmBQPYdW-bq--ei3M7ayp-OnVme8UC_rkV4vI_ea68ejo9k6YVcxAqqqJ_O-xCRyrpmB2JXDsEOWZYFjhiJbSqGZg0SGnVB53bE1kLQqUSWaGT2bjO1V8zLkUFWYmLUFl-F4kgVszcecS8ALkq3FxNGqii66uJgr-wxaXjOmL4qMkErDYzajaETmOGN6LQVfWKfFvxzCC7UCF-oQD6pBwhprRW_Z-AijmPMjoqtUruvKtohQYs4zd_3xHMu06cyRTKpLaKZebjEeN2eMyac0bTgBwBUA8M9-tv0idx2VjWEVZ6B2tXyHENF-VqcCEjALYAsWbBYIKvTRzz1f6Bb3uuOEZrve0npyFCVl9-GjmeryqIEUSM0A_elWShTWAjr_aBXaKFz-5ClV1X2xXEwmdAqCeBjsA9XAdPc3OtIyKcL5vS0uyHHWKf0vIqWznC_7vgnCUbiyBkDqZCEiclcPkqfpUfdBoM73xYS45ptZqx3xj2IAEqCWV1LXdlc3QtMTCNBg.m3u8
#EXT-X-MEDIA:TYPE=VIDEO,GROUP-ID="audio_only",NAME="audio_only",AUTOSELECT=NO,DEFAULT=NO
#EXT-X-STREAM-INF:BANDWIDTH=160000,CODECS="mp4a.40.2",VIDEO="audio_only"
https://video-weaver.cdg02.hls.live-video.net/v1/playlist/CrsFo1rY3gspiPG-G2toXdkP8q5ofXVOA5l-4ctpQc6Y-Smc7ojSqR9AFI8I9awvOio1D-a6WA4kM5xgv_I7UFInxFpQ7DlLWSvWvmyYI5A8xEY_c8K7p6L6f3ZHiAikpyFmMgxma49LVU1Nb6gd_x3H2cPaCm5NotB8zYZ_Lx-V72GTGNAcYYvhPfoj67gg3fYoja77F74mKWQNbmcrCnoc-8Ok01zCOD8Cds_HgCnwE21k74Zo3dOo-nQglczdoapD_IvkBB6u7tESOIEiSMzgi9ZYIZ8i4A_FLihwIOn7yhYvRtJjXxfGayhLJ0nSL45VHE5YOdUUTPKBtZaT-3d3GobbPawRE9rXtlqmn0UKWKqB4pMteGg6YJ8O3vzbh5V9DGK2fKFymB8SyTD-_T7MxA7XauAFZ4KDtliIlW_EjpVdBY32tIraQ53p_WM5-U4_XIJQJ4beWbhZlFsPikxE-ADXJ-zkpVd36fjf_Wx9qzXg_YaiDGMDqcHDfxoZML36TmL7duV9TN6Zsq6cJwjlsZzbvJLwPW3R7XBoPb_3jBVbNj44RcdYnNHe3D6T-p8hqu1tbG52sl88_12ycSYiHapMtyfZdlAYKl5e1EqThQ7mNqYeHnxgskwmP9ZpHKUUg_29PvjGt0tUWa0NLKqzjIU2mYF_tZj84z7QzOG4fl9kRt3J7d_lnODbvsakcokCTHSyJwoXDdnLzf0tos_25a7zgcbJY-osixT9CgoF7qhfmxyCP0u9Hhf0EIAUlVglYjY4o58JRAoQbmBsma-3pa85qP4hj3wuBp48O7I_Rbqfh4LeoMnXQ5jd6U1MeQ90y03lq1nL3R5vqqwsGPEJ6PUJz9WQ2QHg92XWXIhKXLdRJLA5GnCdvy_-vqNaTyKygVHejdXBTx30jo6eQloeqs9FdWrL0E5vngjSGgxGMOfGiOFDC452uVEgASoJZXUtd2VzdC0xMI0G.m3u8`

var expectedStreams = []api.Playlist{
	{
		Bandwidth:  3002999,
		Resolution: "1280x720",
		Codecs:     "avc1.4D401F,mp4a.40.2",
		Video:      "720p60",
		FrameRate:  60.000,
		URL:        "https://video-weaver.cdg02.hls.live-video.net/v1/playlist/Ct0FvodoSVcz8-D3EVrtl47CUOow1G47HAi2dqZov7sZznIr9Fzg4MmdDXqjLcaF0wKiBsk3NIEqu6uCChB58-EzEUrOXHMbUV1M-BllBjZW74D29y-8bbBBn8a_10QJW1ECfJ2pa0dR3GDiHIhlh952bzfEjsqElxqhaKMY37JqOKH3K-_jwtEWHiK18DOYm4pU59W-hzyEPk2TbuLjk6FSWn2vbBU4r1v7A_Zomk58UGuxxj4kF9YoWtvMwb7S4mn2Xj8aMJsZ-asHrPiV_kTksmDtPfRz9CRw-XNRFEjj1vn8NQLBgZVFSKhlpG5jI9U_FKMjf2mpqnoQIZC2kNUfpAi7zHPuJD0NAQ5JLS5O1448HfVpc_STYe81NpxEA75L3AHyO3xZBAt7pUTZXI7hjxzt8FMDb74ee0andg51FlRZ0zN4wXdS7c9ie4JX9zY8OQS1Mo9cfdaNfauqn7N-ZsgJaPI1W32gfI0lAkEaVg3MmOJxasQWy-7MRdsf7kKglMIMNxsNyPAIujkzy7jmbBM8orRS7U6uGlt_hxfQIg2523SaWMLXs2co9dlBxCtoWk_JZW5neXJhSrMffgK0oQBUEIuSwpsVNtPPBZmIbmIMtEYZKc18Cp2s65UqzTA1uJoaw1zONchKDpE0NDZ7wNl2h4x6ZKyD1DI-QID4mq_VnrCcu40DzzlE8YDv6vd36rdra5tFIxox0JvX-d-m-kAfljyDlOiAfgH-2J2MuPZP43o9Z6WT1vW4SBb8TZtZ-Mp8X_lj-x0vLFYz-p3Y2__qHkO-nUt-JRwULGq8D-yy1Go6Hln6FYvSaLL2EiiOgtL7bdoIlZf63mR8dM-QxWKVnuCiKwZzR1hGD57Jwbovsk-wrgp54gXJQGz_PBkiF4qvQOgeDaotH_WRlvU42XnJ1oXdvZ8VT0cVg195jPNZfb6OsjHGfjkvbFg41uwHddg4XCsy3bizH3cyCxoMxnlED5WqSf17aburIAEqCWV1LXdlc3QtMTCNBg.m3u8",
	},
	{
		Bandwidth:  1323000,
		Resolution: "852x480",
		Codecs:     "avc1.4D401F,mp4a.40.2",
		Video:      "480p30",
		FrameRate:  30.000,
		URL:        "https://video-weaver.cdg02.hls.live-video.net/v1/playlist/CtUFJ2BaKoMlM3d1K1N9TigpH4iPSfhX2Mdv1j36wZ88zin-F2vlCN3UierFCNvpnwIXh0eiOmqEfNEpCJBto-a_NpxxcywKJW9SXRGJBU_sB4JrihnED_vxu6-w1mdHvxfEXL6D630VZIGzZFLh0Fw-rF71bm_WehZ1KD5NAXqa-gM2AUu1JUULA760wPZp3hr96vXSHJT5lNBQdNDL2PFPHA0cUExXFcK-Tw_c7vXEANClsbIRW5m4lrY3XbYfKVhYksK3QNYVdLmNEs1HzSR-ijOGDWLNDDm8HnNlC2tiEnXVxK0Ta49gzIz-TxfqRfTYbSAo8rYmMqNvpSbWd2sAUAZ458sEuUNht3L5AksJ0iGGpzhCqriQeX7JexbOyi3P8uJowUMHOv1A6PGAd4Ca_aR7I_3SiA7RakdbLFmRhXD4siKKidSRKs7JKEODpB7QKDXar9BVyoC3gTQWOqvuRL3vYXoTgGYeQOtDXSGZrNCxwJTqt2mvfCu5TvkV3dwprqSOaq8fDoLw88y3Q2hB810kSLD8L-8p3s6w-KQdyfxdpTVcwyG2hyDGd-OYVGtAXE9xp5krldEiCauCMj-WHXapQk6ooFPm0j6W3i-eLVsQZnk4uD1xPmAaXGgWPGBa2iPmOjo1VMugk-ct2PApbWB0FqhnyNRmiz6gCh_ZROKNquOFouRdDELRC0-V8GG4kjZbUhihExe4edTWSjJRAxdalkpF3dBcSqjmAElc-_6TDnM8BgebJJWrpJ4uMP_IrcKlfeT7DZgW-wc5XZlFLPGbVqQc6iaIEau8Fo1roKZF5p4hZobL2C2SxJ17FtQeUe6t1VPA3zRuDFH55aJ90gH7B4OV_uDh5NuO3l0YMT6H_oqJqzeMhe9EFIB4oUZlvmpTmIx87Tx1qRqfZl7RBWu4xnQB_g1Dumd50mSCtFAIjD19ff0PY6JN21NnySZ1zrek_w8aDAY6QqcC129usrQk_CABKglldS13ZXN0LTEwjQY.m3u8",
	},
	{
		Bandwidth:  700000,
		Resolution: "640x360",
		Codecs:     "avc1.4D401F,mp4a.40.2",
		Video:      "360p30",
		FrameRate:  30.000,
		URL:        "https://video-weaver.cdg02.hls.live-video.net/v1/playlist/Cs0FZ4wB9jHcVfk5wj7B5shAT5_6ymmZDB7UxM4W_C9qVfYw8wl19DpTReCV5vesuHqjew9UJRMP7pE7F0rvcJbFFFWrrAMNB3lSPPddJwTZQ5HDAkN7dtpQ-cb2qVtSpoejczA7FBnAwYZVG65kht86v6r9T-gJApnasvWBEtL62bsv1RmFJMqo0m3BXnRarwNjwUSV6j5AcVPLvJljkAtDZS6KmUqBi_on0oSfnhX4koLfhlfGrwUP-qFGD2HQJ0Oh_5QNp_U1o8R19fXrs0vVBaNdLPAxWRrJcioay5LaIVANcndDgiMN3GaRBpW5ewOESwOjqYzhR-rwgLgN2zHERJ6hot5__TPwbE6e9SU58qTpTazAYkJ2M2v2r5hx1AjGvq_SVe9Qg898ySYpud8KZU9_L0Ucphfyc12mnZPphCEosQnSTXisWd07E2GfWwZ_wM3OQPw3WdUAbySou4PKSP_nfKNtEuhYfo85ojioRFEDIuMkguHWl36gBfz5lL2UoMZnpXl-Lz5RqPlh9HVehnSRdYyKLZhYs_JQlwBdgYbzNYPZbXB26aadGjZuHnloh6BU4Bj7-2pfPy0HSvlq3VmfGOJuEYF_QGiUDSomxP6LlFwJK2g9By-Z2BWJMUcC_gcqVbatyRrHXTp9TxzRGVnrfYRmodvxDdsGvPHn8STygPU7YleTRNzjXaWV3Dq9fWFvUxsnqJWkXzXRaF2LzgFtNuAQCxERP0Ac33x77zS3Yv_Ap4BbEc52ukutsRzWFXrf9BAYAv784-DrF2wRcSN9_SaQq3wMdpj8_BN_jbOII_695ujdeh50zVX8fILQUoa3eg4b94NX2n0NOsI6O4pxDI8S94CmEr7obeDY_4CHpnTXZJ8Hd0gr_HooulAayXbXYsJ2yenMStBlYXrJICoXjSxToEjqTztb-RLfdGqNPdypxNP5QsjswmS1GgynFCwYgf1ewjnJ5ZMgASoJZXUtd2VzdC0xMI0G.m3u8",
	},
	{
		Bandwidth:  270000,
		Resolution: "284x160",
		Codecs:     "avc1.4D401F,mp4a.40.2",
		Video:      "160p30",
		FrameRate:  30.000,
		URL:        "https://video-weaver.cdg02.hls.live-video.net/v1/playlist/CsUFpHO_XIJIWpJLbecYWOV5f2Avslr3vTsurZoF6gXamlmTORcHoZfhyMTn96f8pTxstX3g9-shpgHIiutUy4zzZZYuIgRpkXqmksJnJh6sM_vO8khf40HK8SGbfyOT-pakE4xJAmcD-GgaXML7-haEsNQs02nsN0R0F9IPvZcBeKKq-86ihtZczCiF-oFKF_RyZe8Ta6NosWi0bXu-ZRWbNr3o3F7DjBMDsx9Wq9EbrNZ-489X2Ey_v6qMmrM8cHf5B_feVhzBz6TVOplANf8lYxSZxHBBH43yResARsKa9ijuapaYLRZ8R9d5mhf3gfNxdOfOcMXNIYY4Vyda7nZFTGdFouXyp0IxgB2nfGgs0Nkgzus6i3VJUd4aCyrPexhkj1GB9NWa0tIpvHDzDW5i0Po0nU-3l_PSGzOgzuKv8zME43w_ibsm9PHpuqbsiNOFz2z6Uhh80ldaSq_9tnQXO1UmBQPYdW-bq--ei3M7ayp-OnVme8UC_rkV4vI_ea68ejo9k6YVcxAqqqJ_O-xCRyrpmB2JXDsEOWZYFjhiJbSqGZg0SGnVB53bE1kLQqUSWaGT2bjO1V8zLkUFWYmLUFl-F4kgVszcecS8ALkq3FxNGqii66uJgr-wxaXjOmL4qMkErDYzajaETmOGN6LQVfWKfFvxzCC7UCF-oQD6pBwhprRW_Z-AijmPMjoqtUruvKtohQYs4zd_3xHMu06cyRTKpLaKZebjEeN2eMyac0bTgBwBUA8M9-tv0idx2VjWEVZ6B2tXyHENF-VqcCEjALYAsWbBYIKvTRzz1f6Bb3uuOEZrve0npyFCVl9-GjmeryqIEUSM0A_elWShTWAjr_aBXaKFz-5ClV1X2xXEwmdAqCeBjsA9XAdPc3OtIyKcL5vS0uyHHWKf0vIqWznC_7vgnCUbiyBkDqZCEiclcPkqfpUfdBoM73xYS45ptZqx3xj2IAEqCWV1LXdlc3QtMTCNBg.m3u8",
	},
	{
		Bandwidth:  160000,
		Resolution: "",
		Codecs:     "mp4a.40.2",
		Video:      "audio_only",
		FrameRate:  0.000,
		URL:        "https://video-weaver.cdg02.hls.live-video.net/v1/playlist/CrsFo1rY3gspiPG-G2toXdkP8q5ofXVOA5l-4ctpQc6Y-Smc7ojSqR9AFI8I9awvOio1D-a6WA4kM5xgv_I7UFInxFpQ7DlLWSvWvmyYI5A8xEY_c8K7p6L6f3ZHiAikpyFmMgxma49LVU1Nb6gd_x3H2cPaCm5NotB8zYZ_Lx-V72GTGNAcYYvhPfoj67gg3fYoja77F74mKWQNbmcrCnoc-8Ok01zCOD8Cds_HgCnwE21k74Zo3dOo-nQglczdoapD_IvkBB6u7tESOIEiSMzgi9ZYIZ8i4A_FLihwIOn7yhYvRtJjXxfGayhLJ0nSL45VHE5YOdUUTPKBtZaT-3d3GobbPawRE9rXtlqmn0UKWKqB4pMteGg6YJ8O3vzbh5V9DGK2fKFymB8SyTD-_T7MxA7XauAFZ4KDtliIlW_EjpVdBY32tIraQ53p_WM5-U4_XIJQJ4beWbhZlFsPikxE-ADXJ-zkpVd36fjf_Wx9qzXg_YaiDGMDqcHDfxoZML36TmL7duV9TN6Zsq6cJwjlsZzbvJLwPW3R7XBoPb_3jBVbNj44RcdYnNHe3D6T-p8hqu1tbG52sl88_12ycSYiHapMtyfZdlAYKl5e1EqThQ7mNqYeHnxgskwmP9ZpHKUUg_29PvjGt0tUWa0NLKqzjIU2mYF_tZj84z7QzOG4fl9kRt3J7d_lnODbvsakcokCTHSyJwoXDdnLzf0tos_25a7zgcbJY-osixT9CgoF7qhfmxyCP0u9Hhf0EIAUlVglYjY4o58JRAoQbmBsma-3pa85qP4hj3wuBp48O7I_Rbqfh4LeoMnXQ5jd6U1MeQ90y03lq1nL3R5vqqwsGPEJ6PUJz9WQ2QHg92XWXIhKXLdRJLA5GnCdvy_-vqNaTyKygVHejdXBTx30jo6eQloeqs9FdWrL0E5vngjSGgxGMOfGiOFDC452uVEgASoJZXUtd2VzdC0xMI0G.m3u8",
	},
}

func TestParseM3U8(t *testing.T) {
	streams := api.ParseM3U8(strings.NewReader(fixture))

	require.Equal(t, expectedStreams, streams)
}

func BenchmarkParseM3U8(b *testing.B) {
	for b.Loop() {
		api.ParseM3U8(strings.NewReader(fixture))
	}
}

func TestGetBestPlaylist(t *testing.T) {
	streams := append([]api.Playlist{
		{
			Bandwidth:  3002999,
			Resolution: "1280x720",
			Codecs:     "avc1.4D401F,mp4a.40.2",
			Video:      "720p60",
			FrameRate:  30.000,
		},
		{
			Bandwidth:  1000,
			Resolution: "1280x720",
			Codecs:     "avc1.4D401F,mp4a.40.2",
			Video:      "720p60",
			FrameRate:  60.000,
		},
	}, expectedStreams...)

	tt := []struct {
		name       string
		constraint api.PlaylistConstraint
		expected   api.Playlist
		expectedOK bool
	}{
		{
			name:       "best quality",
			expected:   expectedStreams[0],
			expectedOK: true,
		},
		{
			name: "best quality with constraint",
			constraint: api.PlaylistConstraint{
				MaxWidth: 640,
			},
			expected:   expectedStreams[2],
			expectedOK: true,
		},
		{
			name: "audio only",
			constraint: api.PlaylistConstraint{
				AudioOnly: true,
			},
			expected:   expectedStreams[4],
			expectedOK: true,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			bestStream, found := api.GetBestPlaylist(streams, tc.constraint)

			require.Equal(t, tc.expected, bestStream)
			require.Equal(t, tc.expectedOK, found)
		})
	}
}
