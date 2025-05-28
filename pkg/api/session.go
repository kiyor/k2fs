package api

import (
	"log"
	// "net/http" // No longer using net/http directly
	// "github.com/gorilla/sessions" // Session handling will be replaced by Fiber's session middleware later
	"github.com/gofiber/fiber/v2"
)

// Original gorilla/sessions store. Commented out as Fiber will use its own session store.
// var store = sessions.NewCookieStore([]byte("CplRFt9vaVrlZJFB")) // Example key, should be from config

// ApiSessionFiber handles setting session values like sort preferences.
// For now, it only acknowledges the parameters. Actual session storage
// will require integrating Fiber's session middleware.
func ApiSessionFiber(c *fiber.Ctx) error {
	sortBy := c.Query("sortby")
	desc := c.Query("desc")

	log.Printf("ApiSessionFiber: Received sortby=%s, desc=%s. (Session storage not yet implemented with Fiber)", sortBy, desc)

	// In a full implementation with Fiber sessions:
	// store := sessions.New() // Get Fiber session store
	// sess, err := store.Get(c)
	// if err != nil {
	//    return NewErrResp(c, fiber.StatusInternalServerError, 1, "Session error: "+err.Error())
	// }
	// if sortBy != "" {
	//    sess.Set("sortby", sortBy)
	// }
	// if desc != "" {
	//    sess.Set("desc", desc)
	// }
	// if err := sess.Save(); err != nil {
	//    return NewErrResp(c, fiber.StatusInternalServerError, 1, "Session save error: "+err.Error())
	// }

	return NewResp(c, "Session parameters acknowledged (actual saving deferred pending Fiber session middleware integration)", nil)
}

// Original apiSession using gorilla/sessions. Commented out.
/*
func apiSession(w http.ResponseWriter, r *http.Request) {
	// Get a session. We're ignoring the error resulted from decoding an
	// existing session: Get() always returns a session, even if empty.
	session, _ := store.Get(r, APP) // APP constant would need to be defined or passed
	// Set some session values.
	q := r.URL.Query()
	for k, v := range q {
		session.Values[k] = v
	}
	// 	log.Println(session.Values["sortby"])
	// Save it before we write to the response/return from the handler.
	err := session.Save(r, w)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		log.Println(err)
		return
	}
	NewResp(w, "ok", nil) // Old NewResp, would need *fiber.Ctx if used
}
*/
