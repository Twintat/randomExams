package ui

import (
	"fmt"
	"log/slog"
	"math/rand"
	"sort"
	"strconv"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
	"github.com/Twintat/randomExams/config"
	"github.com/Twintat/randomExams/data"
	db "github.com/Twintat/randomExams/database"
	"github.com/Twintat/randomExams/ui/layouts"
)

func startRandomExam(g *data.GUI) {
	form := data.NewExam(g)
	newSet(form)
}

func isBook(id widget.TreeNodeID) bool {
	if len(id) < 1 {
		return false
	} else if id[0] == 'b' {
		return true
	}
	return false
}

func chaptersIDs(id widget.TreeNodeID) ([]widget.TreeNodeID, error) {
	stringID := id[1:]
	intID, err := strconv.Atoi(stringID)
	if err != nil {
		return nil, fmt.Errorf("[chaptersIDs] %w", err)
	}
	chapters, err := db.ListChapters(intID)
	if err != nil {
		return nil, fmt.Errorf("[chaptersIDs] %w", err)
	}
	result := []string{}
	for _, chapter := range chapters {
		result = append(result, "c"+strconv.Itoa(chapter.Id))
	}
	return result, nil
}

func isChapter(id widget.TreeNodeID) bool {
	if len(id) == 0 {
		return false
	} else if id[0] == 'c' {
		return true
	}
	return false
}

func exerciseIDs(id widget.TreeNodeID) ([]widget.TreeNodeID, error) {
	realStrID := id[1:]
	realID, err := strconv.Atoi(realStrID)
	if err != nil {
		return nil, err
	}
	exs, err := db.GetExercises(realID)
	if err != nil {
		return nil, err
	}
	result := []string{}
	for _, ex := range exs {
		result = append(result, "e"+strconv.Itoa(ex.Id))
	}
	return result, nil
}

// make set of exercise
func newSet(form *data.Exam) {
	books, err := db.GetBooks()
	if err != nil {
		slog.Error("[newSet]", "error", err)
		dialog.ShowError(err, form.Gui.Window)
	}

	set := data.SetTable{}

	var bookIDs []string
	for _, book := range books {
		bookIDs = append(bookIDs, "b"+strconv.Itoa(book.Id))
	}
	tree := widget.NewTree(
		// given an nodeID get the sub nodeIDs
		func(id widget.TreeNodeID) []widget.TreeNodeID {
			switch {
			case id == "":
				return bookIDs
			case isBook(id):
				chapters, err := chaptersIDs(id)
				if err != nil {
					slog.Error("[newSet]", "error", err)
					dialog.ShowError(err, form.Gui.Window)
				}
				return chapters
			}
			return []string{}
		},
		// bool func to determine if is branch
		func(id widget.TreeNodeID) bool {
			if id == "" {
				return true
			}
			f := id[0]
			if f == 'b' {
				return true
			}
			return false
		},
		// base widget for the tree node
		func(_ bool) fyne.CanvasObject {
			return widget.NewCheck("", nil)
		},
		// set the content for the tree node
		func(id widget.TreeNodeID, isBranch bool, obj fyne.CanvasObject) {
			if len(id) == 0 {
				return
			}
			treeType := id[0]
			realID, err := strconv.Atoi(id[1:])
			if err != nil {
				slog.Error("[newSet]", "error", err)
				dialog.ShowError(err, form.Gui.Window)
			}
			var checkChange func(b bool)
			var text string
			switch treeType {
			case 'b':
				checkChange = func(b bool) {
					set.AddBookID(realID)
				}
				info, err := db.GetBook(realID)
				if err != nil {
					slog.Error("[newSet]", "error", err)
					dialog.ShowError(err, form.Gui.Window)
				}
				text = info.Info
			case 'c':
				checkChange = func(b bool) {
					set.AddBookID(realID)
				}
				info, err := db.GetChapter(realID)
				if err != nil {
					slog.Error("[newSet]", "error", err)
					dialog.ShowError(err, form.Gui.Window)
				}
				text = info.Info
			default:
				checkChange = nil
			}
			if val, ok := obj.(*widget.Check); ok {
				val.SetText(text)
				val.OnChanged = checkChange
			}
		},
	)

	// saveButton := widget.NewButton(
	// 	"salvar",
	// 	func() {
	// 		// save set and go to next form
	// 		// form.Set = set
	// 		// defineExam(form)
	// 	},
	// )
	contButton := widget.NewButton(
		"continuar",
		func() {
			// don't save but continue
			form.Set = set
			pull, err := db.GenExerPull(&form.Set)
			if err != nil {
				slog.Error("[newSet]", "error", err)
				dialog.ShowError(err, form.Gui.Window)
			}
			form.Pull = append(form.Pull, pull...)
			defineExam(form)
		},
	)
	content := container.New(
		&newSetLayout{},
		tree,
		contButton,
	)
	form.Gui.Window.SetContent(content)
}

// chose existen sets
// func latestSets(form *data.Exam) data.ExerciseSets {
// 	return data.ExerciseSets{}
// }

func defineExam(form *data.Exam) {
	entryExNum := widget.NewEntry()
	entryExNum.SetPlaceHolder("quantos exercicios na prova?")
	entryDuration := widget.NewEntry()
	entryDuration.SetPlaceHolder("quanto tempo de prova?")

	contButton := widget.NewButton(
		"continuar",
		func() {
			numEx, err := strconv.Atoi(entryExNum.Text)
			if err != nil {
				slog.Error("[defineExam]", "error", err)
				dialog.ShowError(err, form.Gui.Window)
			}
			form.Num = numEx
			form.Duration = entryDuration.Text
			runExam(form)
		},
	)

	content := container.NewVBox(
		entryExNum,
		entryDuration,
		contButton,
	)
	form.Gui.Window.SetContent(content)
}

func runExam(form *data.Exam) {
	// build a exercise canvas

	list := container.NewVBox()

	chosen := []int{}
beforeLoop:
	for i := 0; i < form.Num; i++ {
		loteryID := rand.Intn(len(form.Pull))
		slog.Info("pulled random exercise", "id", loteryID)

		// test if loteryID already chosen
		for _, ch := range chosen {
			if loteryID == ch {
				// already matched, unchose!!!
				slog.Info("pulled id was already chosen")
				i--
				continue beforeLoop
			}
		}
		chosen = append(chosen, loteryID)

		exCanvas, err := exerciseCont(form.Pull[loteryID])
		if err != nil {
			slog.Error("[runExam]", "error", err)
			dialog.ShowError(err, form.Gui.Window)
		}
		list.Add(exCanvas)
	}
	topBarWidgets := newTimer(form.Gui, form.Duration)
	cont := container.New(
		&layouts.ExamLayout{},
		topBarWidgets.timer,
		topBarWidgets.bar, topBarWidgets.button,
		container.NewVScroll(list),
	)
	form.Gui.Window.SetContent(cont)
}

func exerciseCont(ex data.Exercise) (fyne.CanvasObject, error) {
	cont := container.New(
		&ExerciseExamLayout{},
	)

	order := make([]int, 0, len(ex.Images))
	for k := range ex.Images {
		order = append(order, k)
	}
	sort.Slice(order, func(i, j int) bool {
		return order[i] < order[j]
	})

	for i := range order {
		imagePath, ok := ex.Images[i]
		if !ok {
			return nil, fmt.Errorf("[exerciseCont] image range and map mismatch")
		}
		slog.Info("images loaded", "path", imagePath)
		img := canvas.NewImageFromFile(config.ImagesDirectory() + "/" + imagePath)
		img.FillMode = canvas.ImageFillContain
		cont.Add(img)
	}
	return cont, nil
}
