#!/bin/sh

hget http://ajax.googleapis.com/ajax/libs/jquery/1.4.2/jquery.min.js > jquery.js
echo '//MACHINE GENERATED, DO NOT EDIT
package gx
var jquery =  `
' > ../jquery.go

sed 's/\`/''/g' < jquery.js >>../jquery.go
echo '`' >>../jquery.go
