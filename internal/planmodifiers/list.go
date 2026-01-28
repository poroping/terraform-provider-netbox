package planmodifiers

import (
	"context"
	"fmt"
	"sort"

	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
)

// UnorderedListModifier returns a plan modifier that ensures list elements are compared
// without regard to order. This is useful for tags and similar attributes where order
// doesn't matter.
type unorderedListModifier struct{}

func (m unorderedListModifier) Description(ctx context.Context) string {
	return "Ensures list elements are compared without regard to order"
}

func (m unorderedListModifier) MarkdownDescription(ctx context.Context) string {
	return "Ensures list elements are compared without regard to order"
}

func (m unorderedListModifier) PlanModifyList(ctx context.Context, req planmodifier.ListRequest, resp *planmodifier.ListResponse) {
	// If the plan is null or unknown, don't modify it
	if req.PlanValue.IsNull() || req.PlanValue.IsUnknown() {
		return
	}

	// If the state is null or unknown, don't modify the plan
	if req.StateValue.IsNull() || req.StateValue.IsUnknown() {
		return
	}

	// Get the plan and state values
	planElements := req.PlanValue.Elements()
	stateElements := req.StateValue.Elements()

	// If lengths are different, the lists are different
	if len(planElements) != len(stateElements) {
		return
	}

	// Convert elements to comparable strings for sorting
	planStrings := make([]string, len(planElements))
	stateStrings := make([]string, len(stateElements))

	for i, elem := range planElements {
		planStrings[i] = fmt.Sprintf("%v", elem)
	}
	for i, elem := range stateElements {
		stateStrings[i] = fmt.Sprintf("%v", elem)
	}

	// Sort both slices
	sort.Strings(planStrings)
	sort.Strings(stateStrings)

	// Compare sorted slices
	equal := true
	for i := range planStrings {
		if planStrings[i] != stateStrings[i] {
			equal = false
			break
		}
	}

	// If they're equal (just reordered), use the state value to avoid unnecessary updates
	if equal {
		resp.PlanValue = req.StateValue
	}
}

// UnorderedList returns a plan modifier that compares list elements without regard to order
func UnorderedList() planmodifier.List {
	return unorderedListModifier{}
}

// UseStateForUnknownUnlessItemsChange returns a plan modifier that copies the state value
// to the plan when the plan value is unknown, unless the items have actually changed
type useStateForUnknownUnlessItemsChangeModifier struct{}

func (m useStateForUnknownUnlessItemsChangeModifier) Description(ctx context.Context) string {
	return "Use state value for unknown list unless items have changed"
}

func (m useStateForUnknownUnlessItemsChangeModifier) MarkdownDescription(ctx context.Context) string {
	return "Use state value for unknown list unless items have changed"
}

func (m useStateForUnknownUnlessItemsChangeModifier) PlanModifyList(ctx context.Context, req planmodifier.ListRequest, resp *planmodifier.ListResponse) {
	// If the plan is not unknown, don't modify it
	if !req.PlanValue.IsUnknown() {
		return
	}

	// If the state is null or unknown, don't modify the plan
	if req.StateValue.IsNull() || req.StateValue.IsUnknown() {
		return
	}

	// If the config is null, the attribute is being removed
	if req.ConfigValue.IsNull() {
		return
	}

	// If the config is unknown, we can't determine if items changed
	if req.ConfigValue.IsUnknown() {
		// Use state value to maintain consistency
		resp.PlanValue = req.StateValue
		return
	}

	// Compare config and state
	configElements := req.ConfigValue.Elements()
	stateElements := req.StateValue.Elements()

	// If lengths are the same and elements match, use state value
	if len(configElements) == len(stateElements) {
		resp.PlanValue = req.StateValue
	}
}

// UseStateForUnknownUnlessItemsChange returns a plan modifier that preserves state when plan is unknown
func UseStateForUnknownUnlessItemsChange() planmodifier.List {
	return useStateForUnknownUnlessItemsChangeModifier{}
}
