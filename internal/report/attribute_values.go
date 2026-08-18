package report

// WithoutAttributeValues returns a deep copy of r with Before/After cleared on every
// AttributeChange. Paths and all other report fields are preserved.
func WithoutAttributeValues(r DriftReport) DriftReport {
	out := r
	if r.ProviderVersions != nil {
		out.ProviderVersions = make(map[string]string, len(r.ProviderVersions))
		for key, value := range r.ProviderVersions {
			out.ProviderVersions[key] = value
		}
	}
	if r.Modules != nil {
		out.Modules = append([]ModuleInventory(nil), r.Modules...)
	}
	if r.OutputChanges != nil {
		out.OutputChanges = make([]OutputChange, len(r.OutputChanges))
		for i, change := range r.OutputChanges {
			out.OutputChanges[i] = OutputChange{
				Name:    change.Name,
				Actions: append([]string(nil), change.Actions...),
			}
		}
	}
	if r.Approval != nil {
		approval := *r.Approval
		out.Approval = &approval
	}
	out.ResourceChanges = make([]ResourceChange, len(r.ResourceChanges))
	for i, change := range r.ResourceChanges {
		cloned := change
		cloned.Actions = append([]string(nil), change.Actions...)
		if change.AuditEvents != nil {
			cloned.AuditEvents = append([]AuditEvent(nil), change.AuditEvents...)
		}
		if change.AttributeChanges != nil {
			cloned.AttributeChanges = make([]AttributeChange, len(change.AttributeChanges))
			for j, attr := range change.AttributeChanges {
				cloned.AttributeChanges[j] = AttributeChange{Path: attr.Path}
			}
		}
		out.ResourceChanges[i] = cloned
	}
	return out
}
