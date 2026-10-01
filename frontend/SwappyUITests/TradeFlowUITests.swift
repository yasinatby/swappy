import XCTest

/// End-to-end walk through a full trade against the local dev backend.
/// Requires: `swappy-backend reset` and `SWAPPY_DEV_LOGIN=true swappy-backend serve`.
final class TradeFlowUITests: XCTestCase {
    private var app: XCUIApplication!

    override func setUp() {
        continueAfterFailure = false
        app = XCUIApplication()
        app.launchArguments = ["-resetSession"]
        app.launch()
    }

    private func snapshot(_ name: String) {
        let attachment = XCTAttachment(screenshot: app.screenshot())
        attachment.name = name
        attachment.lifetime = .keepAlways
        add(attachment)
    }

    private func tapTab(_ label: String) {
        app.tabBars.buttons[label].tap()
    }

    func testFullTradeAsAnna() {
        app.buttons["devLoginButton"].tap()
        let anna = app.buttons["devProfile-anna"]
        XCTAssertTrue(anna.waitForExistence(timeout: 10))
        snapshot("02-dev-picker")
        anna.tap()

        // Feed shows Ben's active items.
        let guitar = app.staticTexts["Westerngitarre Yamaha"]
        XCTAssertTrue(guitar.waitForExistence(timeout: 10))
        snapshot("03-feed")

        // Item detail → offer composer → send.
        guitar.tap()
        let offerButton = app.buttons["offerTradeButton"]
        XCTAssertTrue(offerButton.waitForExistence(timeout: 10))
        snapshot("04-item-detail")
        offerButton.tap()
        let myItem = app.buttons["select-Brettspiel Carcassonne"]
        XCTAssertTrue(myItem.waitForExistence(timeout: 10))
        myItem.tap()
        snapshot("05-composer")
        app.buttons["sendOfferButton"].tap()
        XCTAssertTrue(app.staticTexts["Angebot gesendet"].waitForExistence(timeout: 10))
        snapshot("06-offer-sent")
        app.buttons["Fertig"].tap()

        // Incoming offer from Ben (seed): accept it.
        tapTab("Angebote")
        let incoming = app.staticTexts["Von ben"]
        XCTAssertTrue(incoming.waitForExistence(timeout: 10))
        snapshot("07-offers")
        incoming.firstMatch.tap()
        let accept = app.buttons["offerAction-accept"]
        XCTAssertTrue(accept.waitForExistence(timeout: 10))
        snapshot("08-offer-detail-pending")
        accept.tap()
        app.sheets.buttons["Annehmen"].firstMatch.tap()

        // Accepted → confirm handover.
        let confirm = app.buttons["offerAction-confirm"]
        XCTAssertTrue(confirm.waitForExistence(timeout: 10))
        snapshot("09-offer-accepted")
        confirm.tap()
        app.sheets.buttons["Ja, getauscht"].firstMatch.tap()
        XCTAssertTrue(app.staticTexts["Du: Übergabe bestätigt"].waitForExistence(timeout: 10))
        snapshot("10-confirmed")

        // My items: create a new one.
        tapTab("Meine Sachen")
        XCTAssertTrue(app.staticTexts["Im Tausch"].waitForExistence(timeout: 10))
        snapshot("11-my-items")
        app.buttons["addItemButton"].tap()
        let title = app.textFields["itemTitleField"]
        XCTAssertTrue(title.waitForExistence(timeout: 10))
        title.tap()
        title.typeText("Stehlampe aus Messing")
        snapshot("12-editor")
        app.buttons["saveItemMenu"].tap()
        app.buttons["Veröffentlichen"].tap()
        XCTAssertTrue(app.staticTexts["Stehlampe aus Messing"].waitForExistence(timeout: 10))

        tapTab("Profil")
        XCTAssertTrue(app.buttons["Konto löschen"].waitForExistence(timeout: 10))
        snapshot("13-profile")
    }

    func testNoActiveItemBlocksOffer() {
        app.buttons["devLoginButton"].tap()
        let field = app.textFields["Name, z. B. clara"]
        XCTAssertTrue(field.waitForExistence(timeout: 10))
        field.tap()
        field.typeText("neuling\(Int.random(in: 1000 ... 9999))")
        app.buttons["Anlegen und einloggen"].tap()

        let card = app.staticTexts["Zimmerpflanze Monstera"]
        XCTAssertTrue(card.waitForExistence(timeout: 10))
        card.tap()
        app.buttons["offerTradeButton"].tap()
        XCTAssertTrue(app.staticTexts["Erst etwas einstellen"].waitForExistence(timeout: 10))
        snapshot("14-no-active-item")
    }
}
