package main

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/spf13/cobra"
	api "github.com/VrushankPatel/pulsaar/api"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var (
	tuiCmd = &cobra.Command{
		Use:   "tui",
		Short: "Launch the interactive Terminal User Interface (TUI) pod file explorer",
		RunE:  runTUI,
	}
)

func init() {
	tuiCmd.Flags().String("namespace", "default", "Initial namespace")
}

func runTUI(cmd *cobra.Command, args []string) error {
	initialNamespace, _ := cmd.Flags().GetString("namespace")

	clientset, err := getClientset()
	if err != nil {
		return fmt.Errorf("failed to create Kubernetes client: %w", err)
	}

	app := tview.NewApplication()

	// UI Elements
	header := tview.NewTextView().
		SetDynamicColors(true).
		SetTextAlign(tview.AlignCenter).
		SetText(`[purple]____        _
|  _ \ _   _| |___  __ _  __ _ _ __
| |_) | | | | / __|/ _' |/ _' | '__|
|  __/| |_| | \__ \ (_| | (_| | |
|_|    \__,_|_|___/\__,_|\__,_|_| [cyan]POD EXPLORER
[white]Use Arrow Keys / Tab / Enter to navigate. [purple]Ctrl+C [white]to Exit.`)

	nsList := tview.NewList().ShowSecondaryText(false)
	nsList.SetBorder(true).SetTitle("Namespaces").SetTitleColor(tcell.GetColor("cyan"))

	podList := tview.NewList().ShowSecondaryText(false)
	podList.SetBorder(true).SetTitle("Pods").SetTitleColor(tcell.GetColor("cyan"))

	fileTable := tview.NewTable().SetSelectable(true, false)
	fileTable.SetBorder(true).SetTitle("Files").SetTitleColor(tcell.GetColor("purple"))

	statusBar := tview.NewTextView().SetDynamicColors(true)
	statusBar.SetText("[cyan][Tab] [white]Switch Panel  [cyan][Enter] [white]Select/Open  [cyan][Backspace] [white]Go Back  [cyan][V] [white]View File  [cyan][R] [white]Refresh")

	// Global State
	var currentPath = "/"
	var currentConnClose func()
	var client api.PulsaarAgentClient

	// Layout Flexbox
	leftPanel := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(nsList, 0, 1, true).
		AddItem(podList, 0, 2, false)

	mainPanel := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(header, 6, 1, false).
		AddItem(fileTable, 0, 1, false).
		AddItem(statusBar, 1, 1, false)

	layout := tview.NewFlex().
		AddItem(leftPanel, 30, 1, true).
		AddItem(mainPanel, 0, 2, false)

	pages := tview.NewPages().AddPage("main", layout, true, true)

	// Helper: Update file table directory list
	refreshFiles := func() {
		if client == nil {
			return
		}
		fileTable.Clear()
		// Headers
		headers := []string{"Name", "Size", "Type/Mode", "ModTime"}
		for col, h := range headers {
			cell := tview.NewTableCell(h).SetTextColor(tcell.GetColor("cyan")).SetSelectable(false)
			fileTable.SetCell(0, col, cell)
		}

		resp, err := client.ListDirectory(context.Background(), &api.ListRequest{
			Path:         currentPath,
			AllowedRoots: []string{},
		})
		if err != nil {
			errCell := tview.NewTableCell(fmt.Sprintf("Error listing directory: %v", err)).
				SetTextColor(tcell.GetColor("red")).
				SetSelectable(false)
			fileTable.SetCell(1, 0, errCell)
			return
		}

		// Show special parent directory row if not at root
		rowOffset := 1
		if currentPath != "/" && currentPath != "." {
			parentCell := tview.NewTableCell("..").SetTextColor(tcell.GetColor("purple"))
			fileTable.SetCell(rowOffset, 0, parentCell)
			fileTable.SetCell(rowOffset, 1, tview.NewTableCell("-"))
			fileTable.SetCell(rowOffset, 2, tview.NewTableCell("dir"))
			fileTable.SetCell(rowOffset, 3, tview.NewTableCell("-"))
			rowOffset++
		}

		for idx, entry := range resp.Entries {
			row := idx + rowOffset
			nameColor := tcell.GetColor("white")
			typeStr := "file"
			if entry.IsDir {
				nameColor = tcell.GetColor("purple")
				typeStr = "dir"
			}
			fileTable.SetCell(row, 0, tview.NewTableCell(entry.Name).SetTextColor(nameColor))
			fileTable.SetCell(row, 1, tview.NewTableCell(fmt.Sprintf("%d bytes", entry.SizeBytes)))
			fileTable.SetCell(row, 2, tview.NewTableCell(typeStr))
			fileTable.SetCell(row, 3, tview.NewTableCell(entry.Mtime.AsTime().Format("2006-01-02 15:04:05")))
		}
		fileTable.ScrollToBeginning()
		fileTable.Select(rowOffset, 0)
	}

	// Helper: Show error modal
	showError := func(message string) {
		modal := tview.NewModal().
			SetText(message).
			AddButtons([]string{"OK"}).
			SetDoneFunc(func(buttonIndex int, buttonLabel string) {
				pages.RemovePage("error")
				app.SetFocus(leftPanel)
			})
		pages.AddPage("error", modal, true, true)
	}

	// Helper: Load file contents and show inside a viewer
	viewFile := func(fileName string) {
		filePath := filepath.Join(currentPath, fileName)
		statusBar.SetText(fmt.Sprintf("[yellow]Loading %s...", filePath))
		app.Draw()

		resp, err := client.ReadFile(context.Background(), &api.ReadRequest{
			Path:         filePath,
			Offset:       0,
			Length:       0, // read up to max
			AllowedRoots: []string{},
		})
		if err != nil {
			statusBar.SetText("[cyan][Tab] [white]Switch Panel  [cyan][Enter] [white]Select/Open  [cyan][Backspace] [white]Go Back  [cyan][V] [white]View File  [cyan][R] [white]Refresh")
			showError(fmt.Sprintf("Failed to read file: %v", err))
			return
		}

		viewer := tview.NewTextView().
			SetDynamicColors(false).
			SetText(string(resp.Data))
		viewer.SetBorder(true).
			SetTitle(fmt.Sprintf(" Viewing: %s ", filePath)).
			SetTitleColor(tcell.GetColor("cyan"))

		viewer.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
			if event.Key() == tcell.KeyEscape || event.Rune() == 'q' {
				pages.RemovePage("viewer")
				app.SetFocus(fileTable)
				statusBar.SetText("[cyan][Tab] [white]Switch Panel  [cyan][Enter] [white]Select/Open  [cyan][Backspace] [white]Go Back  [cyan][V] [white]View File  [cyan][R] [white]Refresh")
			}
			return event
		})

		pages.AddPage("viewer", viewer, true, true)
		app.SetFocus(viewer)
	}

	// Load pods for namespace
	loadPods := func(ns string) {
		podList.Clear()
		pods, err := clientset.CoreV1().Pods(ns).List(context.Background(), metav1.ListOptions{})
		if err != nil {
			showError(fmt.Sprintf("Failed to list pods in %s: %v", ns, err))
			return
		}
		for _, pod := range pods.Items {
			podName := pod.Name
			podList.AddItem(podName, "", 0, func() {
				currentPath = "/"

				// Cleanup old connection
				if currentConnClose != nil {
					currentConnClose()
				}

				statusBar.SetText(fmt.Sprintf("[yellow]Connecting to %s/%s agent...", ns, podName))
				app.Draw()

				// Connect to selected pod agent
				conn, cleanup, err := connectToAgent(cmd, podName, ns)
				if err != nil {
					statusBar.SetText("[cyan][Tab] [white]Switch Panel  [cyan][Enter] [white]Select/Open  [cyan][Backspace] [white]Go Back  [cyan][V] [white]View File  [cyan][R] [white]Refresh")
					showError(fmt.Sprintf("Connection failed: %v", err))
					return
				}

				currentConnClose = func() {
					cleanup()
					_ = conn.Close()
				}

				client = api.NewPulsaarAgentClient(conn)
				statusBar.SetText("[cyan][Tab] [white]Switch Panel  [cyan][Enter] [white]Select/Open  [cyan][Backspace] [white]Go Back  [cyan][V] [white]View File  [cyan][R] [white]Refresh")
				refreshFiles()
				app.SetFocus(fileTable)
			})
		}
	}

	// Populate namespace list
	namespaces, err := clientset.CoreV1().Namespaces().List(context.Background(), metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("failed to list namespaces: %w", err)
	}

	initialIdx := 0
	for idx, ns := range namespaces.Items {
		nsName := ns.Name
		if nsName == initialNamespace {
			initialIdx = idx
		}
		nsList.AddItem(nsName, "", 0, func() {
			loadPods(nsName)
			app.SetFocus(podList)
		})
	}
	nsList.SetCurrentItem(initialIdx)
	loadPods(initialNamespace)

	// Switch Panel Handling (Tab key)
	app.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyTab {
			switch app.GetFocus() {
			case nsList:
				app.SetFocus(podList)
			case podList:
				if client != nil {
					app.SetFocus(fileTable)
				} else {
					app.SetFocus(nsList)
				}
			case fileTable:
				app.SetFocus(nsList)
			}
			return nil
		}
		return event
	})

	// File table input handling
	fileTable.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		row, _ := fileTable.GetSelection()
		if row < 1 {
			return event
		}

		nameCell := fileTable.GetCell(row, 0)
		typeCell := fileTable.GetCell(row, 2)
		if nameCell == nil {
			return event
		}

		name := nameCell.Text
		isDir := typeCell.Text == "dir"

		if event.Key() == tcell.KeyEnter {
			if name == ".." {
				// Go up
				currentPath = filepath.Dir(currentPath)
				refreshFiles()
			} else if isDir {
				// Go down
				currentPath = filepath.Join(currentPath, name)
				refreshFiles()
			} else {
				// View file
				viewFile(name)
			}
			return nil
		}

		if event.Key() == tcell.KeyBackspace || event.Key() == tcell.KeyBackspace2 {
			if currentPath != "/" && currentPath != "." {
				currentPath = filepath.Dir(currentPath)
				refreshFiles()
			}
			return nil
		}

		if event.Rune() == 'v' || event.Rune() == 'V' {
			if !isDir && name != ".." {
				viewFile(name)
			}
			return nil
		}

		if event.Rune() == 'r' || event.Rune() == 'R' {
			refreshFiles()
			return nil
		}

		return event
	})

	// Ensure cleanup is done on application exit
	defer func() {
		if currentConnClose != nil {
			currentConnClose()
		}
	}()

	if err := app.SetRoot(pages, true).EnableMouse(true).Run(); err != nil {
		return fmt.Errorf("TUI application failed: %w", err)
	}

	return nil
}
