- the orders should restart at 1 for each day's fish fry. we want them to stay in the database for historical analysis, but they should render as starting at 1. this might mean that the id needs to have a date component that's stripped before rendering in the ui.
- when an order is successfully created, the total should be shown to the user alongside the order number.
- when on admin page, the add menu item button creates the form at the bottom of the page below all of the collapsibles, and pushing it multiple times creates the form multiple times.
- when a new menu item is created and it is included in an order, it is rendered as its uuid.
- the created column on the orders page is showing UTC and timestamp format. it should be local time and a little more readable.
- I get the following errors about websockets in the deployed service `orders.js:7 Mixed Content: The page at 'https://stmichaelfishfry.com/orders' was loaded over HTTPS, but attempted to connect to the insecure WebSocket endpoint 'ws://stmichaelfishfry.com/ws/orders'. This request has been blocked; this endpoint must be available over WSS.
(anonymous)	@	orders.js:7

orders.js:7 Uncaught SecurityError: Failed to construct 'WebSocket': An insecure WebSocket connection may not be initiated from a page loaded over HTTPS.
    at HTMLDocument.<anonymous> (orders.js:7:16)
(anonymous)	@	orders.js:7`
