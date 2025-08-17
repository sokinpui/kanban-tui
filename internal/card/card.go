package card

type Card struct {
	Title string
}

func New(title string) Card {
	return Card{Title: title}
}
