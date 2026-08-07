package auth

import "strings"

// Realistic identities for the simulated MyID registry.
//
// The simulation must not LOOK simulated (plan §10): a demo that returns
// "Verified User AB1234567" as an official full name reads as a stub the
// moment it hits the screen. So the registry returns a plausible Uzbek name in
// the official Surname Name Patronymic order — the same shape as the contract
// example ("Karimov Aziz Baxtiyorovich").
//
// Gender comes from the PINFL itself, using the real rule: the first digit
// encodes century and sex — 1/2 = 1800s (m/f), 3/4 = 1900s (m/f), 5/6 = 2000s
// (m/f). Odd is male, even is female. So a PINFL always produces a
// gender-consistent name, which is exactly what a real registry would do.
//
// (Unrelated to the licence trigger, which reads the LAST digit.)

var (
	maleStems = []string{
		"Karim", "Rashid", "Yusup", "Toshmat", "Nazar", "Abdulla",
		"Ergash", "Sulton", "Xolmat", "Qodir", "Umar", "Said",
	}
	maleNames = []string{
		"Aziz", "Botir", "Sherzod", "Jasur", "Otabek", "Bekzod",
		"Sardor", "Javohir", "Ulugbek", "Farrux", "Doniyor", "Rustam",
	}
	femaleNames = []string{
		"Dilnoza", "Nilufar", "Gulnora", "Zilola", "Malika", "Sevara",
		"Kamola", "Nodira", "Shahnoza", "Feruza", "Zarina", "Muattar",
	}
	patronymicStems = []string{
		"Baxtiyor", "Alisher", "Rustam", "Shavkat", "Zafar", "Oybek",
		"Ilhom", "Nodir", "Sanjar", "Timur", "Ravshan", "Anvar",
	}
)

// isFemalePINFL reads the sex digit of a PINFL (even first digit = female).
func isFemalePINFL(pinfl string) bool {
	if pinfl == "" {
		return false
	}
	d := pinfl[0]
	if d < '0' || d > '9' {
		return false
	}
	return (d-'0')%2 == 0
}

// SimulatedFullName builds the official-form name the registry holds for a
// PINFL. Deterministic: the same person always comes back with the same name.
func SimulatedFullName(pinfl string) string {
	next := derive(pinfl, "name")
	surnameStem := maleStems[next(len(maleStems))]
	patronymicStem := patronymicStems[next(len(patronymicStems))]

	if isFemalePINFL(pinfl) {
		return strings.Join([]string{
			surnameStem + "ova",
			femaleNames[next(len(femaleNames))],
			patronymicStem + "ovna",
		}, " ")
	}
	return strings.Join([]string{
		surnameStem + "ov",
		maleNames[next(len(maleNames))],
		patronymicStem + "ovich",
	}, " ")
}
