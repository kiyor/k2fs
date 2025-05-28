package api

import (
	"encoding/json"
	"fmt"
	"log"
	// "net/http" // No longer using net/http directly
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	"github.com/gofiber/fiber/v2"
	"github.com/kiyor/k2fs/pkg/core"     // For core.GlobalAppConfig, core.TrashPath
	"github.com/kiyor/k2fs/pkg/lib"      // For lib.Cache
	kfs "github.com/kiyor/k2fs/pkg/lib" // Alias for kfs types
)

// ApiOperationRequest defines the structure for operation API requests.
type ApiOperationRequest struct {
	Files  map[string]bool `json:"files"`
	Dir    string          `json:"dir"`   // Path relative to rootDir
	Action string          `json:"action"`
}

// ActionKey extracts the main action type (e.g., "delete", "mark").
func (o *ApiOperationRequest) ActionKey() string {
	return strings.Split(o.Action, "=")[0]
}

// ActionValue extracts the value for actions like "mark=value".
func (o *ApiOperationRequest) ActionValue() string {
	s := strings.Split(o.Action, "=")
	if len(s) > 1 {
		return s[1]
	}
	return ""
}

// opMutex ensures that file operations are serialized.
var opMutex *sync.Mutex = new(sync.Mutex)

// ApiOperationFiber handles various file operations like delete, mark, label, etc.
func ApiOperationFiber(c *fiber.Ctx) error {
	opMutex.Lock()
	defer opMutex.Unlock()

	req := new(ApiOperationRequest)
	if err := c.BodyParser(req); err != nil {
		return NewErrResp(c, fiber.StatusBadRequest, 1, "Invalid request for operation: "+err.Error())
	}

	// currentDirPath is the absolute path to the directory where operations are performed.
	currentDirPath := filepath.Join(core.GlobalAppConfig.RootDir, req.Dir)

	// meta is for the .kfs file in the current directory (legacy metadata).
	meta := kfs.NewMeta(currentDirPath) // kfs.NewMeta expects absolute path

	for fileHashOrName, selected := range req.Files {
		if !selected {
			continue // Skip files not marked true in the map
		}

		// Assuming fileHashOrName is the HASH of the file/directory.
		// We need the relative path of the item from metaV2.
		// The original code used 'k' (map key) as the filename for filepath.Join(path, k)
		// and also for meta.Get(k). This implies 'k' was a filename, not a hash.
		// If 'k' is indeed a filename (e.g., "video.mp4", "subdir/"), then:
		itemFileName := fileHashOrName 
		itemAbsPath := filepath.Join(currentDirPath, itemFileName) // Absolute path on disk
		
		// itemRelPathToRootDir is the path relative to rootDir, used as key in metaV2.
		itemRelPathToRootDir := filepath.Join(req.Dir, itemFileName)


		// log.Printf("Processing operation '%s' for item: %s (abs: %s, rel: %s)", req.Action, itemFileName, itemAbsPath, itemRelPathToRootDir)

		// m is the legacy metadata from .kfs file.
		m, _ := meta.Get(itemFileName) 
		
		// m2 is the new metadata from metaV2 (presumably BoltDB/Redis).
		m2, err := metaV2.Get(itemRelPathToRootDir) // metaV2 uses path relative to rootDir
		if err != nil {
			// Attempt to load if not found in cache, might be a new file not yet indexed by background tasks.
			// metaV2.LoadPath was not a standard method in original meta_v2.go.
			// This suggests direct indexing or that Get should handle loading if not found.
			// For now, if Get fails, we log and continue, as original did.
			log.Printf("ApiOperationFiber: metaV2.Get error for key '%s': %v. Skipping item.", itemRelPathToRootDir, err)
			continue
		}
		
		// Note: The original code had `m2, err = metaV2.LoadPath(key)` which is not standard.
		// Assuming `metaV2.Get` is sufficient or `LoadPath` needs to be part of `MetaV2` interface.
		// For now, we rely on `metaV2.Get`.

		switch {
		case req.Action == "unzip":
			// Unzip logic requires server-side commands, ensure paths are safe.
			// This is a potentially dangerous operation if paths are not strictly controlled.
			var cmdToRun string
			var cmd *exec.Cmd
			targetDir := filepath.Dir(itemAbsPath)
			baseName := filepath.Base(itemAbsPath)
			extractToDir := filepath.Join(targetDir, baseName[:len(baseName)-len(filepath.Ext(baseName))]) // Remove extension

			if err := os.Mkdir(extractToDir, 0755); err != nil {
				log.Printf("ApiOperationFiber: Unzip mkdir error for %s: %v", extractToDir, err)
				// return NewErrResp(c, fiber.StatusInternalServerError, 1, "Unzip mkdir error: "+err.Error())
				continue // Or fail operation? Original returned.
			}
			switch filepath.Ext(strings.ToLower(itemAbsPath)) {
			case ".rar":
				cmdToRun = fmt.Sprintf("unrar -y e '%s' '%s'", baseName, extractToDir) // Note: extractToDir should be absolute or relative to Dir
				cmd = exec.Command("/bin/sh", "-c", cmdToRun)
			case ".zip":
				cmdToRun = fmt.Sprintf("unzip '%s' -d '%s'", baseName, extractToDir)
				cmd = exec.Command("/bin/sh", "-c", cmdToRun)
			default:
				log.Printf("ApiOperationFiber: Unsupported archive type for unzip: %s", itemAbsPath)
				continue
			}
			cmd.Dir = targetDir // Run command in the file's directory
			log.Printf("ApiOperationFiber: Unzip running cmd: cd %s && %s", targetDir, cmdToRun)
			cmd.Stderr = os.Stderr // Consider capturing output instead of direct pipe
			cmd.Stdout = os.Stdout
			if errCmd := cmd.Run(); errCmd != nil {
				log.Printf("ApiOperationFiber: Unzip cmd error for %s: %v", itemAbsPath, errCmd)
				// Potentially clean up extractToDir if cmd failed.
			}

		case strings.HasPrefix(req.Action, "label"): // Legacy label
			labelValue := req.ActionValue()
			m.Label = labelValue // For .kfs
			meta.Set(itemFileName, m)
			m2.SetLabel(labelValue) // For metaV2

		case strings.HasPrefix(req.Action, "mark"):
			markValue := req.ActionValue()
			switch markValue {
			case "5": // Star and label danger
				m2.SetLabel("danger"); m2.SetStar(true)
				m.Label = "danger"; m.Star = true; meta.Set(itemFileName, m)
			case "4": // Label danger
				m2.SetLabel("danger")
				m.Label = "danger"; meta.Set(itemFileName, m)
			default:
				// Clear label if markValue is empty or not 4/5? Original didn't specify.
				// Or set label to markValue directly? E.g. mark=customLabel
				// Current logic only handles 4 and 5 for "mark".
				// For safety, if it's not 4 or 5, we can assume it's a direct label set.
				if markValue != "" {
					m2.SetLabel(markValue)
					m.Label = markValue; meta.Set(itemFileName, m)
				} else { // mark= (empty) -> clear label
					m2.SetLabel("")
					m.Label = ""; meta.Set(itemFileName, m)
				}
			}
		
		case strings.HasPrefix(req.Action, "icons"): // Legacy
			iconsValue := req.ActionValue()
			if iconsValue != "" {
				m.Icons = []string{iconsValue}
			} else {
				m.Icons = []string{}
			}
			meta.Set(itemFileName, m)
			// No direct m2.SetIcons equivalent shown, handle if necessary

		case strings.HasPrefix(req.Action, "star"): // Handles "star" and "star=true/false"
			starValue := true // Default for "star" action
			actionSpecificValue := req.ActionValue()
			if actionSpecificValue == "false" {
				starValue = false
			} else if actionSpecificValue == "true" {
				starValue = true
			} else if req.Action == "star" { // Toggle if just "star"
				starValue = !m2.GetStar() // Use m2's current state for toggle
			}
			m2.SetStar(starValue)
			m.Star = starValue; meta.Set(itemFileName, m)


		case req.Action == "restore":
			// itemAbsPath is currently path inside currentDir. If currentDir is Trash, then itemAbsPath is in Trash.
			// m.OldLoc should be path relative to rootDir.
			if strings.HasPrefix(req.Dir, strings.TrimLeft(core.TrashPath, "./\\")) || req.Dir == core.TrashPath { // Check if current dir is Trash
				oldLocAbs := filepath.Join(core.GlobalAppConfig.RootDir, m.OldLoc) // m.OldLoc is path relative to rootDir
				log.Printf("ApiOperationFiber: Restore %s from Trash to %s", itemAbsPath, oldLocAbs)
				
				// Ensure destination directory exists
				if err := os.MkdirAll(filepath.Dir(oldLocAbs), 0755); err != nil {
					log.Printf("ApiOperationFiber: Restore mkdir error for %s: %v", filepath.Dir(oldLocAbs), err)
					continue
				}
				if err := os.Rename(itemAbsPath, oldLocAbs); err != nil {
					log.Printf("ApiOperationFiber: Restore os.Rename error %s to %s: %v", itemAbsPath, oldLocAbs, err)
					continue
				}
				
				// Update metadata in destination
				// Assuming m.OldLoc is like "movies/file.mkv" (relative to root)
				// And itemFileName for .kfs is just "file.mkv"
				destDirRel := filepath.Dir(m.OldLoc)
				destDirAbs := filepath.Join(core.GlobalAppConfig.RootDir, destDirRel)
				destMeta := kfs.NewMeta(destDirAbs) // .kfs meta in original directory
				destMeta.Set(filepath.Base(m.OldLoc), m) // Use original filename for .kfs key
				if err := destMeta.Write(); err != nil {
					log.Printf("ApiOperationFiber: Restore destMeta.Write error: %v", err)
				}

				meta.Del(itemFileName) // Delete from .Trash/.kfs
				
				// metaV2: move item from Trash path to original path
				// itemRelPathToRootDir is path in Trash (e.g. ".Trash/file.mkv")
				// m.OldLoc is original path relative to root (e.g. "movies/file.mkv")
				if err := metaV2.Move(itemRelPathToRootDir, m.OldLoc); err != nil {
					log.Printf("ApiOperationFiber: Restore metaV2.Move error from %s to %s: %v", itemRelPathToRootDir, m.OldLoc, err)
				}
				// No need to SetLabel or SetStar as Move should preserve existing meta or it's restored.

				if lib.Cache != nil { lib.Cache.Remove("size:.Trash") }
			}


		case req.Action == "delete":
			trashMeta := kfs.NewMeta(filepath.Join(core.GlobalAppConfig.RootDir, core.TrashPath)) // .kfs meta for trash dir

			if itemAbsPath == filepath.Join(core.GlobalAppConfig.RootDir, core.TrashPath) { // Request to delete .Trash directory itself
				log.Printf("ApiOperationFiber: Clearing all files in Trash: %s", core.TrashPath)
				trashDirEntries, err := os.ReadDir(filepath.Join(core.GlobalAppConfig.RootDir, core.TrashPath))
				if err != nil {
					log.Printf("ApiOperationFiber: Error reading .Trash for clearing: %v", err)
					continue
				}
				for _, entry := range trashDirEntries {
					fullPathInTrash := filepath.Join(core.GlobalAppConfig.RootDir, core.TrashPath, entry.Name())
					if err := os.RemoveAll(fullPathInTrash); err != nil {
						log.Printf("ApiOperationFiber: Error removing %s from .Trash: %v", fullPathInTrash, err)
					}
				}
				// Clear .kfs in trash? Original didn't explicitly clear trashMeta entries here.
			} else if strings.HasPrefix(req.Dir, strings.TrimLeft(core.TrashPath, "./\\")) || req.Dir == core.TrashPath) { // Item is inside Trash, permanent delete
				log.Printf("ApiOperationFiber: Permanently deleting from Trash: %s", itemAbsPath)
				if err := os.RemoveAll(itemAbsPath); err != nil {
					log.Printf("ApiOperationFiber: os.RemoveAll error for %s: %v", itemAbsPath, err)
					continue
				}
				meta.Del(itemFileName) // Remove from .Trash/.kfs
				metaV2.Del(itemRelPathToRootDir) // Remove from metaV2 (path is like .Trash/item)
			} else { // Item is not in Trash, move to Trash
				destInTrashAbsPath := filepath.Join(core.GlobalAppConfig.RootDir, core.TrashPath, itemFileName)
				log.Printf("ApiOperationFiber: Moving to Trash: %s -> %s", itemAbsPath, destInTrashAbsPath)
				
				// Ensure .Trash directory itself exists
				if err := os.MkdirAll(filepath.Join(core.GlobalAppConfig.RootDir, core.TrashPath), 0755); err != nil {
					log.Printf("ApiOperationFiber: Error creating .Trash directory %s: %v", core.TrashPath, err)
					continue
				}

				if err := os.Rename(itemAbsPath, destInTrashAbsPath); err != nil {
					log.Printf("ApiOperationFiber: os.Rename to Trash error for %s: %v", itemAbsPath, err)
					// Try to recover by setting label to delete if move failed? Or just error out.
					// For now, continue to next file if move fails.
					continue 
				}
				
				meta.Del(itemFileName) // Delete from source_dir/.kfs
				
				m.OldLoc = itemRelPathToRootDir // Store original path (relative to rootDir)
				trashMeta.Set(itemFileName, m) // Add to .Trash/.kfs with OldLoc
				
				// metaV2: move item to Trash path, effectively renaming its key
				// itemRelPathToRootDir is original relative path (e.g. "movies/file.mkv")
				// destRelPathInTrash is path inside trash (e.g. ".Trash/file.mkv")
				destRelPathInTrash := filepath.Join(core.TrashPath, itemFileName)
				if err := metaV2.Move(itemRelPathToRootDir, destRelPathInTrash); err != nil {
					log.Printf("ApiOperationFiber: Delete metaV2.Move error from %s to %s: %v", itemRelPathToRootDir, destRelPathInTrash, err)
				}
				// Optionally, also set a "deleted" label or flag in metaV2 for the item in Trash, if needed.
				// m2.SetLabel("deleted_from_"+req.Dir) // Example
			}
			
			if err := trashMeta.Write(); err != nil { // Write .Trash/.kfs
				log.Printf("ApiOperationFiber: trashMeta.Write error: %v", err)
			}
			if lib.Cache != nil { lib.Cache.Remove("size:.Trash") } // Invalidate cache for trash size
			
			// These were in original, might be too broad or slow for single op.
			// Consider if they are necessary after every delete, or handled by background tasks.
			// metaV2.RemoveOrphan(core.TrashPath) 
			// metaV2.Index(core.TrashPath)
			// dirSize2(core.TrashPath) // Update trash size in cache
		}
		// After action, save metaV2 changes for the item
		if errSave := metaV2.Set(m2); errSave != nil {
			log.Printf("ApiOperationFiber: metaV2.Set error for key '%s': %v", itemRelPathToRootDir, errSave)
		}
	}

	if err := meta.Write(); err != nil { // Write current_dir/.kfs
		log.Printf("ApiOperationFiber: current dir meta.Write error: %v", err)
		return NewErrResp(c, fiber.StatusInternalServerError, 1, "Error writing metadata: "+err.Error())
	}

	return NewResp(c, "Operation successful", nil)
}
