//go:build linux && (amd64 || arm64)

package linux

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"unsafe"

	"github.com/ebitengine/purego"
	"github.com/egoist/mygo/internal/platform"
)

type dialogs struct{ b *Backend }

const gtkResponseAccept = -3

func parentWindow(pw platform.Window) ptr {
	if w, ok := pw.(*window); ok && w != nil && !w.closed {
		return w.win
	}
	return 0
}

// slistStrings reads and frees a GSList of GLib allocated strings.
func slistStrings(list ptr) []string {
	var out []string
	for node := list; node != 0; node = field[ptr](node, 8) {
		out = append(out, takeStr(field[ptr](node, 0)))
	}
	if list != 0 {
		gSlistFree(list)
	}
	return out
}

func setupChooser(fc ptr, defaultPath string, filters []platform.FileFilter, hidden, save bool) {
	gtkFileChooserSetShowHidden(fc, hidden)
	if defaultPath != "" {
		if st, err := os.Stat(defaultPath); err == nil && st.IsDir() {
			gtkFileChooserSetCurrentFolder(fc, cs(defaultPath))
		} else {
			dir := filepath.Dir(defaultPath)
			gtkFileChooserSetCurrentFolder(fc, cs(dir))
			if !save && gtkFileChooserSelectFilename != nil {
				gtkFileChooserSelectFilename(fc, cs(defaultPath))
			}
			if save {
				gtkFileChooserSetCurrentName(fc, cs(filepath.Base(defaultPath)))
			}
		}
	}
	for _, f := range filters {
		filter := gtkFileFilterNew()
		name := f.Name
		if name == "" {
			name = strings.Join(append(slices.Clip(f.Extensions), f.MIMETypes...), ", ")
		}
		gtkFileFilterSetName(filter, cs(name))
		for _, e := range f.Extensions {
			e = strings.TrimPrefix(e, ".")
			if e == "*" {
				gtkFileFilterAddPattern(filter, cs("*"))
				continue
			}
			gtkFileFilterAddPattern(filter, cs("*."+e))
			if up := strings.ToUpper(e); up != e {
				gtkFileFilterAddPattern(filter, cs("*."+up))
			}
		}
		// GTK matches "kind/*" itself, and the portal's file chooser
		// takes the types as they are.
		for _, m := range f.MIMETypes {
			gtkFileFilterAddMimeType(filter, cs(m))
		}
		gtkFileChooserAddFilter(fc, filter)
	}
}

func setupImagePreview(fc ptr) {
	loadMenuIcons()
	if gtkFileChooserSetPreviewWidget == nil || gtkBoxNew == nil || gtkImageNew == nil || gdkPixbufNewFromFileAtScale == nil {
		return
	}

	box := gtkBoxNew(1, 8)
	if gtkWidgetSetSizeRequest != nil {
		gtkWidgetSetSizeRequest(box, 300, -1)
	}
	if gtkWidgetSetValign != nil {
		gtkWidgetSetValign(box, 1) // GTK_ALIGN_START
	}
	if gtkWidgetSetMarginStart != nil {
		gtkWidgetSetMarginStart(box, 12)
		gtkWidgetSetMarginEnd(box, 12)
		gtkWidgetSetMarginTop(box, 16)
		gtkWidgetSetMarginBottom(box, 16)
	}

	image := gtkImageNew()
	if gtkBoxPackStart != nil {
		gtkBoxPackStart(box, image, false, false, 6)
	}

	lblName := gtkLabelNew(nil)
	if gtkLabelSetLineWrap != nil {
		gtkLabelSetLineWrap(lblName, true)
	}
	if gtkBoxPackStart != nil {
		gtkBoxPackStart(box, lblName, false, false, 4)
	}

	lblMeta := gtkLabelNew(nil)
	if gtkBoxPackStart != nil {
		gtkBoxPackStart(box, lblMeta, false, false, 2)
	}

	if gtkWidgetShowAll != nil {
		gtkWidgetShowAll(box)
	}

	gtkFileChooserSetPreviewWidget(fc, box)
	if gtkFileChooserSetUsePreviewLabel != nil {
		gtkFileChooserSetUsePreviewLabel(fc, false)
	}

	cb := purego.NewCallback(func(chooser, user_data ptr) {
		if gtkFileChooserGetPreviewFilename == nil {
			return
		}
		fnPtr := gtkFileChooserGetPreviewFilename(chooser)
		if fnPtr == 0 {
			if gtkFileChooserSetPreviewWidgetActive != nil {
				gtkFileChooserSetPreviewWidgetActive(chooser, false)
			}
			return
		}
		fn := takeStr(fnPtr)
		if fn == "" {
			if gtkFileChooserSetPreviewWidgetActive != nil {
				gtkFileChooserSetPreviewWidgetActive(chooser, false)
			}
			return
		}
		st, err := os.Stat(fn)
		if err != nil || st.IsDir() {
			if gtkFileChooserSetPreviewWidgetActive != nil {
				gtkFileChooserSetPreviewWidgetActive(chooser, false)
			}
			return
		}

		var w, h int32
		var fmtName string
		fnCs := cs(fn)
		if gdkPixbufGetFileInfo != nil {
			fmtPtr := gdkPixbufGetFileInfo(fnCs, &w, &h)
			if fmtPtr != 0 && gdkPixbufFormatGetName != nil {
				namePtr := gdkPixbufFormatGetName(fmtPtr)
				if namePtr != nil {
					var s []byte
					for p := namePtr; *p != 0; p = (*byte)(unsafe.Pointer(uintptr(unsafe.Pointer(p)) + 1)) {
						s = append(s, *p)
					}
					fmtName = string(s)
				}
			}
		}

		pb := gdkPixbufNewFromFileAtScale(fnCs, 280, 280, true, nil)
		if pb == 0 {
			if gtkFileChooserSetPreviewWidgetActive != nil {
				gtkFileChooserSetPreviewWidgetActive(chooser, false)
			}
			return
		}

		if gtkImageSetFromPixbuf != nil {
			gtkImageSetFromPixbuf(image, pb)
		}
		if gObjectUnref != nil {
			gObjectUnref(pb)
		}

		baseName := filepath.Base(fn)
		if gtkLabelSetText != nil {
			gtkLabelSetText(lblName, cs(baseName))
		}

		sz := st.Size()
		var szStr string
		if sz < 1024 {
			szStr = fmt.Sprintf("%d B", sz)
		} else if sz < 1024*1024 {
			szStr = fmt.Sprintf("%.1f KB", float64(sz)/1024)
		} else {
			szStr = fmt.Sprintf("%.1f MB", float64(sz)/(1024*1024))
		}

		metaStr := szStr
		if w > 0 && h > 0 {
			if fmtName != "" {
				metaStr = fmt.Sprintf("%d × %d • %s • %s", w, h, strings.ToUpper(fmtName), szStr)
			} else {
				metaStr = fmt.Sprintf("%d × %d • %s", w, h, szStr)
			}
		}
		if gtkLabelSetText != nil {
			gtkLabelSetText(lblMeta, cs(metaStr))
		}

		if gtkFileChooserSetPreviewWidgetActive != nil {
			gtkFileChooserSetPreviewWidgetActive(chooser, true)
		}
	})

	if gSignalConnectData != nil {
		gSignalConnectData(fc, cs("update-preview"), cb, 0, 0, 0)
	}
}

func (d dialogs) ShowOpenDialog(parent platform.Window, o *platform.OpenDialogOptions, cb func([]string, error)) {
	tx := platform.CurrentTexts()
	action := int32(0) // GTK_FILE_CHOOSER_ACTION_OPEN
	title := tx.Open
	if o.OpenDirectories {
		action, title = 2, tx.SelectFolder // SELECT_FOLDER
	}
	if o.Title != "" {
		title = o.Title
	}
	accept := "_" + tx.Open
	if o.ButtonLabel != "" {
		accept = o.ButtonLabel
	}
	fc := gtkFileChooserNativeNew(cs(title), parentWindow(parent), action, cs(accept), cs("_"+tx.Cancel))
	gtkFileChooserSetSelectMultiple(fc, o.Multiple)
	setupChooser(fc, o.DefaultPath, o.Filters, o.ShowHiddenFiles, false)
	if !o.OpenDirectories {
		setupImagePreview(fc)
	}
	var paths []string
	if d.b.runDialog(fc, true) == gtkResponseAccept {
		paths = slistStrings(gtkFileChooserGetFilenames(fc))
	}
	gObjectUnref(fc)
	cb(paths, nil)
}

func (d dialogs) ShowSaveDialog(parent platform.Window, o *platform.SaveDialogOptions, cb func(string, error)) {
	tx := platform.CurrentTexts()
	title := o.Title
	if title == "" {
		title = tx.Save
	}
	accept := "_" + tx.Save
	if o.ButtonLabel != "" {
		accept = o.ButtonLabel
	}
	fc := gtkFileChooserNativeNew(cs(title), parentWindow(parent), 1, cs(accept), cs("_"+tx.Cancel)) // SAVE
	gtkFileChooserSetDoOverwriteConfirm(fc, true)
	setupChooser(fc, o.DefaultPath, o.Filters, o.ShowHiddenFiles, true)
	path := ""
	if d.b.runDialog(fc, true) == gtkResponseAccept {
		path = takeStr(gtkFileChooserGetFilename(fc))
	}
	gObjectUnref(fc)
	cb(path, nil)
}

func (d dialogs) ShowMessageBox(parent platform.Window, o *platform.MessageBoxOptions, cb func(platform.MessageBoxResult, error)) {
	kind := int32(4) // GTK_MESSAGE_OTHER
	switch o.Type {
	case "info":
		kind = 0
	case "warning":
		kind = 1
	case "question":
		kind = 2
	case "error":
		kind = 3
	}
	message := o.Message
	if message == "" {
		message = o.Title
	}
	dlg := gtkMessageDialogNew(parentWindow(parent), 1|2, kind, 0, cs("%s"), cs(message)) // MODAL | DESTROY_WITH_PARENT, BUTTONS_NONE
	if o.Detail != "" {
		gtkMessageDialogFormatSecondaryText(dlg, cs("%s"), cs(o.Detail))
	}
	if o.Title != "" {
		gtkWindowSetTitle(dlg, cs(o.Title))
	}
	for i, label := range o.Buttons {
		btn := gtkDialogAddButton(dlg, cs(label), int32(i))
		if slices.Contains(o.DestructiveIDs, i) {
			gtkStyleContextAddClass(gtkWidgetGetStyleContext(btn), cs("destructive-action"))
		}
	}
	gtkDialogSetDefaultResponse(dlg, int32(o.DefaultID))
	var check ptr
	if o.CheckboxLabel != "" {
		check = gtkCheckButtonNewWithLabel(cs(o.CheckboxLabel))
		gtkToggleButtonSetActive(check, o.CheckboxChecked)
		gtkContainerAdd(gtkMessageDialogGetMessageArea(dlg), check)
		gtkWidgetShow(check)
	}
	resp := d.b.runDialog(dlg, false)
	res := platform.MessageBoxResult{Response: int(resp)}
	if resp < 0 {
		// Escape or the window's close button.
		res.Response = o.CancelID
	}
	if check != 0 {
		res.CheckboxChecked = gtkToggleButtonGetActive(check)
	}
	gtkWidgetDestroy(dlg)
	cb(res, nil)
}
