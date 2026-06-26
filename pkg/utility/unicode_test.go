package utility_test

import (
	"testing"

	"github.com/cuvou/gosocial/pkg/utility"
)

func TestNormalizeUnicode(t *testing.T) {
	var tests = []struct {
		In     string
		Expect string
	}{
		{
			"Hello world",
			"Hello world",
		},
		{
			"Têstôöłæ",
			"Testoolae",
		},
		{
			"Düzce, Düzce, TR",
			"Duzce, Duzce, TR",
		},
		{
			"Lübeck, Schleswig-Holstein, DE",
			"Lubeck, Schleswig-Holstein, DE",
		},
		{
			"Joünié, Mont-Liban, LB",
			"Jounie, Mont-Liban, LB",
		},
		{
			"Böblingen, Baden-Württemberg, DE",
			"Boblingen, Baden-Wurttemberg, DE",
		},
		{
			"Priozërsk, Leningradskaya Oblast’, RU",
			"Priozersk, Leningradskaya Oblast', RU",
		},
		{
			"Ísafjörður",
			"Isafjorour",
		},
		{
			"Höfn, Sveitarfélagið Hornafjörður, IS",
			"Hofn, Sveitarfelagio Hornafjorour, IS",
		},
		{
			"Ţālkhvoncheh, Eşfahān, IR",
			"Talkhvoncheh, Esfahan, IR",
		},
		{
			"Nowshahr, Māzandarān, IR",
			"Nowshahr, Mazandaran, IR",
		},
		{
			"Vaux-le-Pénil, Île-de-France, FR",
			"Vaux-le-Penil, Ile-de-France, FR",
		},
		{
			"Strängnäs, Södermanland, SE",
			"Strangnas, Sodermanland, SE",
		},
		{
			"Norrtälje, Stockholm, SE",
			"Norrtalje, Stockholm, SE",
		},
		{
			"Åhus, Skåne, SE",
			"Ahus, Skane, SE",
		},
		{
			"Örnsköldsvik, Västernorrland, SE",
			"Ornskoldsvik, Vasternorrland, SE",
		},
		{
			"Borås, Västra Götaland, SE",
			"Boras, Vastra Gotaland, SE",
		},
		{
			"Pochëp, Bryanskaya Oblast’, RU",
			"Pochep, Bryanskaya Oblast', RU",
		},
		{
			"Usol’ye-Sibirskoye, Irkutskaya Oblast’, RU",
			"Usol'ye-Sibirskoye, Irkutskaya Oblast', RU",
		},
		{
			"Pénjamo, Guanajuato, MX",
			"Penjamo, Guanajuato, MX",
		},

		// Problematic ones.
		{
			// The z̄ rune doesn't decode correctly in Go.
			"Herīs, Āz̄arbāyjān-e Sharqī, IR",
			"Heris, Az̄arbayjan-e Sharqi, IR",
		},
		{
			"Nāz̧erābād, Yazd, IR",
			"Naz̧erabad, Yazd, IR",
		},
	}

	for i, test := range tests {
		actual := utility.NormalizeUnicode(test.In)

		if actual != test.Expect {
			t.Errorf(
				"Test #%d: expected '%s' but got '%s'",
				i, test.Expect, actual,
			)
		}
	}
}
