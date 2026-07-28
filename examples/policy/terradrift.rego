package main

is_production if {
	contains(lower(input.directory), "prod")
}

is_replacement(change) if {
	count(change.actions) == 2
	"create" in change.actions
	"delete" in change.actions
}

deny contains msg if {
	some change in input.resource_changes
	change.actions == ["delete"]
	msg := sprintf("destructive drift detected for %s", [change.address])
}

deny contains msg if {
	is_production
	some change in input.resource_changes
	is_replacement(change)
	msg := sprintf("replacement drift detected in production for %s", [change.address])
}
