package notify

import (
	"errors"
	"testing"
)

func TestClassOf(t *testing.T) {
	cases := map[string]string{
		"":              ClassMotion,
		"motion":        ClassMotion,
		"Movimiento":    ClassMotion,
		"cat (87%)":     "cat",
		"person":        "person",
		"Person (100%)": "person",
		"dog(55%)":      "dog",
		"horse (40%)":   ClassOther,
		"teddy bear":    ClassOther,
		"other":         ClassOther,
	}
	for label, want := range cases {
		if got := ClassOf(label); got != want {
			t.Errorf("ClassOf(%q) = %q, want %q", label, got, want)
		}
	}
}

func TestShouldNotifyByMode(t *testing.T) {
	s := Settings{Mode: ModeHome, Home: []string{"person"}, Away: []string{"person", "cat", ClassMotion}}

	if s.ShouldNotify("cat (90%)") {
		t.Error("at home, cats must not notify")
	}
	if !s.ShouldNotify("person (80%)") {
		t.Error("at home, people must notify with this config")
	}

	s.Mode = ModeAway
	for _, l := range []string{"cat (90%)", "person", "motion"} {
		if !s.ShouldNotify(l) {
			t.Errorf("away: %q must notify", l)
		}
	}
	if s.ShouldNotify("dog (70%)") {
		t.Error("away: dog is not in the list")
	}
}

func TestDefaults(t *testing.T) {
	d := Defaults()
	if d.Mode != ModeAway || len(d.Home) != 0 || len(d.Away) != len(Classes) {
		t.Fatalf("unexpected defaults: %+v", d)
	}
	if !d.ShouldNotify("horse (30%)") {
		t.Error("away default must notify everything, including other classes")
	}
}

func TestValidate(t *testing.T) {
	s := Settings{Mode: "party", Home: []string{}, Away: []string{}}
	if err := s.Validate(); !errors.Is(err, ErrInvalidSettings) {
		t.Error("invalid mode accepted")
	}
	s = Settings{Mode: ModeHome, Home: []string{"Person", "person", " cat "}, Away: []string{}}
	if err := s.Validate(); err != nil {
		t.Fatal(err)
	}
	if len(s.Home) != 2 || s.Home[0] != "person" || s.Home[1] != "cat" {
		t.Errorf("classes not normalized/deduplicated: %v", s.Home)
	}
	s = Settings{Mode: ModeHome, Home: []string{"dinosaur"}}
	if err := s.Validate(); err == nil {
		t.Error("unknown class accepted")
	}
}
