/*
	Serve a window system made out of multiple wax interfaces.
*/
package index

import (
	"clive/net/wax"
	"fmt"
	"net/http"
)

func ServeAt(at string, urls []string) {
	http.HandleFunc(at, func(w http.ResponseWriter, r *http.Request) {
		js := `
		<html>
		<body>
		<script type="text/javascript" src="/js/jq/jquery.js"></script>
		<p>
		<script>
		$(function(){
			function xdrop(e) {
				console.log('pos', xx, cx, yy, cy, xx-cx, yy-cy);
				console.log(dragid);
				if(!document.zztop)
					document.zztop = 1;
				else
					document.zztop = document.zztop+1;
				$('#'+dragid)
					.css('top', yy-cy)
					.css('left', xx-cx)
					.css('position', 'fixed')
					.css('zIndex', document.zztop)
					.click(function() {
						$('#'+this.id).css('position', 'static')
						.css('zIndex', document.zztop);

						document.zztop++;
					});
			}
	
			document.body.ondrop = xdrop;
			document.body.ondragover = function(e) {
				xx = e.x;
				yy = e.y;
				console.log(e.clientX, e.clientY);
				e.preventDefault();
			}
	
			$('.wax').each(function(){
				$(this).attr('draggable', true);
				$(this).bind('dragstart', function(e) {
					document.zztop++;
					$(this).css('zIndex', document.zztop);
					console.log(e);
					if(!e.target) {
						console.log("no id");
					} else {
						dragid = e.target.id;
						var pos = $('#'+e.target.id).position();
						cx = e.originalEvent.x - pos.left;
						cy = e.originalEvent.y - pos.top;
						console.log("->", e.target.id, cx, cy);
					}
				});
			});
		});
		</script>`

		fmt.Fprintf(w, "%s\n<p>\n", js)
		if !wax.Auth(w, r) {
			return
		}
		for i := range urls {
			wid := fmt.Sprintf("wax%d", i)
			js = `<div id="` + wid + `" class="wax" resize="both"`
			js += ` style="background-color:#f3f3f3; border:1px solid black; width:400px; height:200px; resize:both; overflow: hidden; scrolling:no;">
			<div id ="` + wid + `tag">` + urls[i] + `:</div>` + "\n"
			js += `<iframe id="` + wid + `if" src="` + urls[i] + `"`
			js += ` frameborder="0"  style="resize:none; height:95%; padding:0; width:98%; margin: 0px;">
</iframe>
</div>
`
			fmt.Fprintf(w, "%s\n<p>\n", js)
			js = `$(function(){
				var tag = $("#` + wid + `if").contents().find(".hastag");
				console.log("tag", tag);
				if(tag && tag.tag){
					console.log("tag", tag.tag);
					$("#` + wid + `tag").get(0).text(tag.tag);
				}
			});`
			fmt.Fprintf(w, "<script>\n%s\n</script>\n", js)
		}
		fmt.Fprintf(w, "<p></body></html>\n")
	})
}
