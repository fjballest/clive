package ink


import (
	"clive/cmd"
	"clive/net/auth"
	"fmt"
	"net/http"
	"clive/x/code.google.com/p/go.net/websocket"
	"strings"
)

func authFailed(w http.ResponseWriter, r *http.Request) {
	outs := `
		<script>
		document.cookie = "clive=xxx; expires=Thu, 01 Jan 1970 00:00:01 GMT;";
		</script>
		<b>Authentication failed</b>
		<p><p><center><b><tt>
		<b>Please, proceed to the <a href="/login">login page</a>.
		</tt></b></center><p><p>
	`
	fmt.Fprintf(w, "%s\n", outs)
}

func checkOrigin(config *websocket.Config, req *http.Request) (err error) {
	config.Origin, err = websocket.Origin(config, req)
	if err == nil && config.Origin == nil {
		return fmt.Errorf("null origin")
	}
	return err
}
// Authenticate a websocket before servicing it.
func AuthWebSocketHandler(h websocket.Handler) http.HandlerFunc {
	hndler := func(w http.ResponseWriter, r *http.Request) {
		if auth.TLSserver != nil && auth.Enabled {
			clive, err := r.Cookie("clive")
			if err != nil {
				cmd.Warn("wax/auth: no cookie: %s", err)
				http.Error(w, "auth failed", 403)
				return
			}
			toks := strings.SplitN(string(clive.Value), ":", 2)
			if len(toks) < 2 {
				cmd.Warn("wax/auth: wrong cookie")
				http.Error(w, "auth failed", 403)
				return
			}
			ch, resp := toks[0], toks[1]
			u, ok := auth.ChallengeResponseOk("wax", ch, resp)
			if !ok {
				cmd.Warn("wax/auth: failed for %s", u)
				http.Error(w, "auth failed", 403)
				return
			}
		}
		s := websocket.Server{Handler: h, Handshake: checkOrigin}
		s.ServeHTTP(w, r)
	}
	return hndler
}

// Authenticate before calling the handler.
// When TLS is disabled, or there's no key file, auth is considered ok.
func AuthHandler(fn http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if auth.TLSserver==nil || !auth.Enabled {
			fn(w, r)
			return
		}
		clive, err := r.Cookie("clive")
		if err != nil {
			cmd.Warn("wax/auth: no cookie: %s", err)
			authFailed(w, r)
			return
		}
		toks := strings.SplitN(string(clive.Value), ":", 2)
		if len(toks) < 2 {
			cmd.Warn("wax/auth: wrong cookie")
			authFailed(w, r)
			return
		}
		ch, resp := toks[0], toks[1]
		u, ok := auth.ChallengeResponseOk("wax", ch, resp)
		if !ok {
			cmd.Warn("wax/auth: failed for %s", u)
			authFailed(w, r)
			return
		}
		fn(w, r)
	}
}

func ServeLoginFor(proceedto string) {
	http.HandleFunc("/login", func(w http.ResponseWriter, r *http.Request) {
		vals := r.URL.Query()
		if len(vals["dst"]) > 0 {
			proceedto = vals["dst"][0]
		}
		js := `
		<html>
		<body>
		<script type="text/javascript" src="/js/aes.js"></script>
		<script type="text/javascript" src="/js/ansix923.js"></script>
		<script type="text/javascript" src="/js/pbkdf2.js"></script>
		<script type="text/javascript" src="/js/jquery-2.2.0.js"></script>
		<p>
		<script>
		$(function(){
			$("#dialog").on('submit', function(e) {
				var salt ='ltsa';
				var usrkey = $("#pass").val();
				var key = CryptoJS.PBKDF2(usrkey, salt, { keySize: 256/32, iterations: 1000});
				usrkey = "XXXXXXXXXXXX";
				var ch = Math.random().toPrecision(16).slice(2);
				var iv  = CryptoJS.enc.Hex.parse('12131415161718191a1b1c1d1e1f1011');
				var enc  = CryptoJS.AES.encrypt(ch, key, { iv: iv, padding: CryptoJS.pad.Pkcs7});
				var c =  "clive=" + ch + ":" + enc.ciphertext + ";secure=secure";
				document.cookie = c;
				clive = c;
				window.location = "` + proceedto + `";
				return false;
			});
		})
		if(window.location.protocol !== "https:") {
			window.location = "` + proceedto + `";
		}
		</script>
		<p><center><b><tt>
		<form name="form" id="dialog" action="" method="get" >
			<label for="box">Clive ink password: </label>
			<input name="box" id="pass" type="password"/ ></form></tt></b></center>
`
		fmt.Fprintf(w, "%s\n<p>\n", js)
		fmt.Fprintf(w, `</body></html>`+"\n")
	})
} 

