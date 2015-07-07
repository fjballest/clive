/*
	Wax is a user interface system that relies on HTML5
	to provide user interfaces for both go and native commands.
	Highly experimental, just like everything else.

	A wax interface is one or more pages made out of one or more
	wax interface parts.

	A part is an HTML template bound to one or more go
	entities, served as a wax page.

	The part template contains HTML with wax command marked within
	$ characters.

	The part is applied to an environment that binds identifiers
	used in wax commands to go values, and then served within a page.

	The simplest command is $name$, where name is a (go) identifier
	as provided by the environment. This presents the corresponding go
	value in the interface.

	To present a go value, it all depends on what data type it has:

	- Simple data types are presented by printing their value.

	- Complex data types are presented by a hierarchy of items for
	their members. The format of the items depends on the default
	format set for the wax part (eg., Itemizer or Divider).

	- Values implementing Presenter are presented by calling their
	ShowAt() method.

	- Values implementing Controller (also implement Presenter) are
	presented by calling ShowAt() and have events and updates conveyed
	to/from the user interface from/to the value channels.

	If the named identifier contains fields (slices, structures, and
	maps), the syntax $name[id1].id2[id3]...$ can be used to present
	the field of interest.

	The iteration command:

		$for id in name do$
			...
		$end$

	generates HTML for the body where id is defined in the
	environment for each value found in name (name names an entity
	in the environment, can be use field access syntax).

	The selection command

		$if name do$
			...
		$else$
			...
		$end$

	generates HTML for the then-arm or the else-arm depending on
	the value of name considered as a boolean.

	Note that in compound commands it is ok if we write them as

		$for id in name do
			...
		end$

	as long as the body contains only wax commands and not HTML.



*/
package wax
