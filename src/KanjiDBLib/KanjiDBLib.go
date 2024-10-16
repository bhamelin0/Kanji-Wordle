package KanjiDBLib

import (
	"bufio"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"os"
	"slices"
	"strconv"
	"strings"
)

// Collection of helper functionality for defining interactions with the kanji postgres db
const KanjiFileNames = "../data/JLPT Kanji/N"
const entryCloseTag = "</entry>"
const vocabTag = "<keb>"
const readingTag = "<reb>"
const glossTag = "<gloss>"
const priTag = "<ke_pri>"
const fileName = "../data/JMDict_e"

type KanjiOfDay struct {
	Kanji           string
	Kanji_id        int
	VocabCollection []Vocab
}

type DailyKanji struct {
	Kanji    string
	Kanji_id int
	N_level  int
}

type Vocab struct {
	Vocab_id int
	Vocab    string
	Common   bool
	Readings []string
	Gloss    []string
}

// Public functions

// Returns an object including all vocabs of a kanji for parsing to json and sending to UI
func GetKanjiOfDayObj(db *sql.DB, kanji string) KanjiOfDay {
	return getKanjiOfDayHelper(db, kanji)
}

func GetKanjiDailyListObj(db *sql.DB) []DailyKanji {
	var dailyKanjiList = []DailyKanji{}
	rows, err := db.Query(DailyKanji_Select)
	if err != nil {
		fmt.Println(err)
	}

	for rows.Next() {
		var kanji DailyKanji
		var dump int
		if err := rows.Scan(&kanji.Kanji_id, &kanji.Kanji, &kanji.N_level, &dump); err != nil {
			log.Fatal(err)
		}

		dailyKanjiList = append(dailyKanjiList, kanji)
	}

	return dailyKanjiList
}

// Should determine new kanji for the day and update all data so it will be gotten
func InitKanjiOfDay(db *sql.DB) {

}

// Takes a kanji string and uploads entire list of vocab gloss and readings to DB
func InitVocabForKanji(db *sql.DB, kanji string) {
	vocabList := findVocabForKanji(kanji)
	for _, vocab := range vocabList {
		uploadVocabToDb(db, kanji, vocab)
	}
}

// Iterates through all kanji and connects associated vocab
// TODO: Improve speed by iterating through all vocab one by one and finding associated Kanji in SQL instead
func InitVocabForAllKanji(db *sql.DB) {
	AllKanji := getAllKanji()
	fmt.Println("Total kanji" + strconv.Itoa((len(AllKanji))))
	for index, kanji := range AllKanji {
		fmt.Println("Running for Kanji #" + strconv.Itoa((index)))
		vocabList := findVocabForKanji(kanji)
		for _, vocab := range vocabList {
			uploadVocabToDb(db, kanji, vocab)
		}
	}
}

func InitializeNewKanjiJitsuDB(db *sql.DB) {
	fmt.Println("Initializing DB")
	_, err := db.Exec(initDBSQL)
	if err != nil {
		fmt.Println(err)
	}
	fmt.Println("DB initialized")
}

func PopulateKanjiTable(db *sql.DB) {
	var sb strings.Builder
	sb.WriteString(InsertKanjiTableSQL)
	first := true

	for i := 1; i <= 5; i++ {
		file, err := os.Open(KanjiFileNames + strconv.Itoa(i))
		if err != nil {
			log.Fatal(err)
		}

		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			newKanji := scanner.Text()
			if first {
				first = false
				sb.WriteString("('" + newKanji + "', " + strconv.Itoa(i) + ") ")
			} else {
				sb.WriteString(", ('" + newKanji + "', " + strconv.Itoa(i) + ")")
			}
		}
	}
	sb.WriteString(";")
	fmt.Println(sb.String())

	_, err := db.Exec(sb.String())
	if err != nil {
		fmt.Println(err)
	}
}

// Updates Kanji Of Day
func UpdateDailyKanji(db *sql.DB) (kanji []int) {
	kanjiIDs := getFiveKanjiHelper(db)

	if len(kanjiIDs) != 5 {
		_, err := db.Exec(DailyKanji_Reset)
		if err != nil {
			fmt.Println(err)
		}
		kanjiIDs = getFiveKanjiHelper(db)
	}
	if len(kanjiIDs) != 5 {
		log.Fatal("Cannot retrieve 5 kanji; Failing")
	}

	_, err := db.Exec(DailyKanji_Expire)
	if err != nil {
		fmt.Println(err)
	}

	_, err = db.Exec(DailyKanji_SetDaily, kanjiIDs[0], kanjiIDs[1], kanjiIDs[2], kanjiIDs[3], kanjiIDs[4])
	if err != nil {
		fmt.Println(err)
	}

	return kanjiIDs
}

// Private functions

func getAllKanji() (kanji []string) {
	var kanjiList = []string{}

	for i := 1; i <= 5; i++ {
		file, err := os.Open(KanjiFileNames + strconv.Itoa(i))
		if err != nil {
			log.Fatal(err)
		}

		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			kanjiList = append(kanjiList, scanner.Text())
		}
	}
	return kanjiList
}

func getFiveKanjiHelper(db *sql.DB) []int {
	rows, err := db.Query(DailyKanji_GetFiveNew)
	if err != nil {
		fmt.Println(err)
	}

	kanjiIds := []int{}

	for rows.Next() {
		var (
			id int
		)
		if err := rows.Scan(&id); err != nil {
			log.Fatal(err)
		}

		kanjiIds = append(kanjiIds, id)
	}
	return kanjiIds
}

// Return obj containing vocab, readings, and gloss entries. Expects scanner to be on current vocab text
func getEntryData(scanner *bufio.Scanner) (Vocab, error) {
	newVocabtext := parseTagText(scanner.Text())

	newVocab := Vocab{Vocab: newVocabtext, Readings: []string{}, Gloss: []string{}}

	for scanner.Scan() {
		text := scanner.Text()
		if strings.HasPrefix(text, readingTag) {
			newVocab.Readings = append(newVocab.Readings, parseTagText(text))
		}
		if strings.HasPrefix(text, glossTag) {
			newVocab.Gloss = append(newVocab.Gloss, parseTagText(text))
		}
		if strings.HasPrefix(text, priTag) {
			newVocab.Common = true
		}

		if strings.HasPrefix(text, entryCloseTag) {
			if len(newVocab.Readings) > 0 && len(newVocab.Gloss) > 0 {
				return newVocab, nil
			} else {
				return newVocab, errors.New("no readings")
			}
		}
	}

	return newVocab, errors.New("no readings")
}

func getKanjiOfDayHelper(db *sql.DB, kanji string) KanjiOfDay {
	// Get the core vocab data
	rows, err := db.Query(selectKanjiVocabSQL, kanji)
	if err != nil {
		fmt.Println(err)
	}

	kanjiOfday := KanjiOfDay{Kanji: kanji}
	var kanji_id int

	for rows.Next() {
		var (
			vocab    string
			vocab_id int
			common   bool
		)
		if err := rows.Scan(&kanji_id, &vocab, &vocab_id, &common); err != nil {
			log.Fatal(err)
		}

		newVocab := Vocab{Vocab: vocab, Vocab_id: vocab_id, Common: common, Gloss: []string{}, Readings: []string{}}
		kanjiOfday.VocabCollection = append(kanjiOfday.VocabCollection, newVocab)
	}
	kanjiOfday.Kanji_id = kanji_id
	rows.Close()

	// Get the glosses and append them into the matching vocab datas
	glossRows, err := db.Query(selectGlossSQL, kanji_id)
	if err != nil {
		fmt.Println(err)
	}

	for glossRows.Next() {
		var (
			vocab_id int
			gloss    string
		)
		if err := glossRows.Scan(&vocab_id, &gloss); err != nil {
			log.Fatal(err)
		}

		matchingVocabEntryIndex := slices.IndexFunc(kanjiOfday.VocabCollection, func(n Vocab) bool {
			return n.Vocab_id == vocab_id
		})
		kanjiOfday.VocabCollection[matchingVocabEntryIndex].Gloss = append(kanjiOfday.VocabCollection[matchingVocabEntryIndex].Gloss, gloss)
	}
	glossRows.Close()

	// Get the readings and append them into the matching vocab datas
	readRows, err := db.Query(selectReadingSQL, kanji_id)
	if err != nil {
		fmt.Println(err)
	}

	for readRows.Next() {
		var (
			vocab_id int
			reading  string
		)
		if err := readRows.Scan(&vocab_id, &reading); err != nil {
			log.Fatal(err)
		}

		matchingVocabEntryIndex := slices.IndexFunc(kanjiOfday.VocabCollection, func(n Vocab) bool {
			return n.Vocab_id == vocab_id
		})
		kanjiOfday.VocabCollection[matchingVocabEntryIndex].Readings = append(kanjiOfday.VocabCollection[matchingVocabEntryIndex].Readings, reading)
	}
	readRows.Close()

	return kanjiOfday
}

// Helper to move scanner to next vocab
func findNextVocab(scanner *bufio.Scanner, kanji string) bool {
	for scanner.Scan() {
		if strings.HasPrefix(scanner.Text(), vocabTag) && strings.Contains(scanner.Text(), kanji) {
			return true
		}
	}
	return false // End of file
}

// Returns array of vocab from the dictionary for a given kanji string
func findVocabForKanji(kanji string) []Vocab {
	file, err := os.Open(fileName)
	if err != nil {
		log.Fatal(err)
	}

	scanner := bufio.NewScanner(file)

	vocabList := []Vocab{}

	for findNextVocab(scanner, kanji) {
		newVocab, error := getEntryData(scanner)
		if error == nil {
			vocabList = append(vocabList, newVocab)
		}
	}
	return vocabList
}

func parseTagText(text string) string {
	noStartTag := strings.Split(text, ">")[1]
	noEndTag := strings.Split(noStartTag, "<")[0]
	return noEndTag
}

// Uploads a single vocab to the DB, assigning it to the kanji and saving all readings and glosses.
func uploadVocabToDb(db *sql.DB, kanji string, vocab Vocab) {

	// Vocab and relation to Kanji
	//fmt.Println(("Uploading ") + kanji)
	rows, err := db.Query(VocabInsertSQL, vocab.Vocab, vocab.Common, kanji)
	if err != nil {
		fmt.Println(err)
	}
	rows.Next()
	var vocab_id int
	if err := rows.Scan(&vocab_id); err != nil {
		fmt.Println(err)
	}
	rows.Close()

	// Glosses
	for _, glossEntry := range vocab.Gloss {
		_, err = db.Exec(SafeGlossInsertSQL, vocab_id, glossEntry, glossEntry)
		if err != nil {
			fmt.Println(err)
			fmt.Println(strconv.Itoa(vocab_id))
			fmt.Println(glossEntry)
		}
	}

	// Readings
	for _, readEntry := range vocab.Readings {
		_, err = db.Exec(SafeReadingInsertSQL, vocab_id, readEntry, readEntry)
		if err != nil {
			fmt.Println(err)
			fmt.Println(strconv.Itoa(vocab_id))
			fmt.Println(readEntry)
		}
	}
}
