package shortcuts

import "testing"

func TestShortcutValidation(t *testing.T) {
	if err := Validate(Defaults()); err != nil {
		t.Fatal(err)
	}
	for name, binding := range map[string]Binding{
		"fixed save":  {Code: "KeyS", Primary: true},
		"no modifier": {Code: "KeyJ"}, "unknown key": {Code: "Escape", Primary: true},
		"editing conflict": {Code: "KeyC", Primary: true}, "browser close": {Code: "KeyW", Primary: true},
		"ambiguous plus": {Code: "Equal", Primary: true, Shift: true},
	} {
		t.Run(name, func(t *testing.T) {
			b := Defaults()
			b["deck.theme"] = binding
			if Validate(b) == nil {
				t.Fatal("accepted invalid shortcut")
			}
		})
	}
	b := Defaults()
	b["deck.theme"] = Binding{Code: "KeyT", Primary: true, Alt: true}
	if err := Validate(b); err != nil {
		t.Fatal(err)
	}
	delete(b, "deck.theme")
	b["other"] = Binding{Code: "KeyB", Primary: true}
	if Validate(b) == nil {
		t.Fatal("accepted unknown action")
	}
	b = Defaults()
	b["trigger.command"] = Binding{Trigger: "a"}
	if Validate(b) == nil {
		t.Fatal("accepted text trigger")
	}
}
