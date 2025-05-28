package api

import (
	"fmt"
	"log"
	// "path/filepath" // Not strictly needed if only using the raw path for title/metaV2

	"github.com/gofiber/fiber/v2"
	// metaV2 is a package variable in 'api' package, initialized in main.go
)

// RenderPhotoFiber handles serving the photo gallery page.
func RenderPhotoFiber(c *fiber.Ctx) error {
	// Get the path parameter. If route is /photo/*, c.Params("*") gets content of *.
	photoPath := c.Params("*") 

	if photoPath == "" {
		log.Println("RenderPhotoFiber: No path provided after /photo/")
		return c.Status(fiber.StatusBadRequest).SendString("No photo path specified.")
	}

	// Construct title (similar to original).
	title := fmt.Sprintf("K2FS - Photo - %s", photoPath)

	// Get photo list data from metaV2.ListPhoto.
	// metaV2 is a package variable in 'api' package, initialized in main.go via SetMetaV2.
	if metaV2 == nil {
		log.Println("RenderPhotoFiber: metaV2 is not initialized")
		return c.Status(fiber.StatusInternalServerError).SendString("Server error: metaV2 not initialized.")
	}
	
	// listPhotoData is assumed to be a string like " 'path1', 'path2' " for JS array
	// This matches the original behavior where ListPhoto returned a string.
	listPhotoDataString, err := metaV2.ListPhoto(photoPath)
	if err != nil {
		log.Printf("RenderPhotoFiber: Error from metaV2.ListPhoto for path '%s': %v", photoPath, err)
		// It's possible ListPhoto returns an error if the path is invalid or no photos found.
		// Depending on desired behavior, could return 404 or an empty gallery.
		// For now, treating errors as server errors.
		return c.Status(fiber.StatusInternalServerError).SendString("Error generating photo list.")
	}

	// Render the photo.html template.
	// The template engine (with delimiters and funcs) is configured in main.go.
	// photo.html should be in the directory specified in html.New() in main.go (e.g., "./").
	// The data passed to template: Title and TheImageListString.
	// The photo.html template should use: [[ .Title ]] and [[ .TheImageListString ]].
	// If TheImageListString is already valid JS (e.g. "'img1.jpg','img2.jpg'"), then
	// in JS: var imglist = [[[ .TheImageListString ]]]; would work if TheImageListString doesn't have HTML unsafe chars.
	// Or, if it needs to be unescaped: var imglist = [ {{- .TheImageListString }} ]; (using Fiber's default {{ }} if not overridden)
	// Since we set Delims("[[", "]]"), it would be: var imglist = [ [[- .TheImageListString ]] ]; for unescaped.
	return c.Render("photo.html", fiber.Map{
		"Title":            title,
		"TheImageListString": listPhotoDataString, 
	})
}
