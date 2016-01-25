package web


import (
	"clive/cmd"
	"clive/net/auth"
	"fmt"
	"net/http"
	"strings"
)

func authFailed(w http.ResponseWriter) {
	outs := `
		<script>
		document.cookie = "clive=xxx; expires=Thu, 01 Jan 1970 00:00:01 GMT;";
		</script>
		<b>Authentication failed</b>
		<p>
		<b>Please, proceed to the <a href="/login">login page</a> to try again.</b>
		<p>
	`
	fmt.Fprintf(w, "%s\n", outs)
}


// Authenticate the client of the interface. To be called early within the
// handler function for clive pages. It returns false if auth failed
// and the handler should return without handling anything.
// When TLS is disabled, or there's no key file, auth is considered ok.
func Auth(w http.ResponseWriter, r *http.Request) bool {
	if auth.TLSserver==nil || !auth.Enabled {
		return true
	}
	clive, err := r.Cookie("clive")
	if err != nil {
		cmd.Warn("wax/auth: no cookie: %s", err)
		authFailed(w)
		return false
	}
	toks := strings.SplitN(string(clive.Value), ":", 2)
	if len(toks) < 2 {
		cmd.Warn("wax/auth: wrong cookie")
		authFailed(w)
		return false
	}
	ch, resp := toks[0], toks[1]
	u, ok := auth.ChallengeResponseOk("wax", ch, resp)
	if !ok {
		cmd.Warn("wax/auth: failed for %s", u)
		authFailed(w)
		return false
	}
	return ok
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
		if(window.location.protocol === "https:") {
			console.log("HTTPS");
			var cookies = []
			var clive = "";
			if(false && document.cookie) {
				cookies = document.cookie.split(";")
				for(var i = 0; i < cookies.length; i++) {
					var args = cookies[i].split("=");
					if(args[0] == "clive"){
						console.log("clive cookie found");
						clive = args[1];
						break;
					}
				}
			}
			if(clive == ""){
				console.log("clive cookie not found");
				var salt ='ltsa';
				var usrkey = prompt("clive wax pass: ");
				var key = CryptoJS.PBKDF2(usrkey, salt, { keySize: 256/32, iterations: 1000});
				usrkey = "XXXXXXXXXXXX";
				var ch = Math.random().toPrecision(16).slice(2);
				var iv  = CryptoJS.enc.Hex.parse('12131415161718191a1b1c1d1e1f1011');
				var enc  = CryptoJS.AES.encrypt(ch, key, { iv: iv, padding: CryptoJS.pad.Pkcs7});
				console.log("ch ", ch);
				console.log("resp ", ""+enc.ciphertext);
				var c =  "clive=" + ch + ":" + enc.ciphertext + ";secure=secure";
				document.cookie = c;
				clive = c;
			}
		}
		window.location = "` + proceedto + `";
		</script>`
		fmt.Fprintf(w, "%s\n<p>\n", js)
		fmt.Fprintf(w, "<p></body></html>\n")
	})
}
