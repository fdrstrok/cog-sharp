package demo

import shared "example.com/shared"

#Child: {
	enabled: *false | bool
}

#First: {
	kind: "first"
	value: string
}

#Second: {
	kind: "second"
	count: int
}

#Keywords: {
	String: string & =~"^\\d+\"?$"
	Bool: bool
	Event: #Child
}

#Dashboard: {
	title: *"CPU" | string
	tags: *[] | [...string]
	child?: #Child
	choice?: #First | #Second
	value?: bool | string
	active?: bool
	count?: int
	remote?: shared.#RemoteChild
}
