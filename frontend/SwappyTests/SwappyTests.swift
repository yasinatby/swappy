import XCTest
@testable import Swappy

final class DecodingTests: XCTestCase {
    func testParsesGoTimestampsWithNanoseconds() throws {
        let date = try XCTUnwrap(DateParsing.parse("2026-10-01T13:12:41.123456789+02:00"))
        XCTAssertEqual(date.timeIntervalSince1970, 1_790_853_161.123, accuracy: 0.001)
        XCTAssertNotNil(DateParsing.parse("2026-10-01T11:12:41Z"))
        XCTAssertNotNil(DateParsing.parse("2026-10-01T11:12:41.5Z"))
        XCTAssertNil(DateParsing.parse("gestern"))
    }

    func testDecodesOfferFromBackend() throws {
        let json = """
        {"id":"f1","initiator_id":"b","responder_id":"a","target_item_id":"a1","message":"Gitarre gegen dein Rad?",
         "status":"pending","expires_at":"2026-10-15T13:12:41.836251+02:00","accepted_at":null,
         "initiator_confirmed_at":null,"responder_confirmed_at":null,"created_at":"2026-10-01T13:12:41.836251+02:00",
         "initiator_name":"ben","responder_name":"anna",
         "items":[{"id":"b1","side":"offered","title":"Gitarre","status":"active","cover_path":null},
                  {"id":"a1","side":"requested","title":"Rennrad","status":"active","cover_path":"x.jpg"}]}
        """
        let offer = try APIClient.decoder.decode(Offer.self, from: Data(json.utf8))
        XCTAssertEqual(offer.status, .pending)
        XCTAssertTrue(offer.isIncoming(for: "a"))
        XCTAssertEqual(offer.counterpartyName(for: "a"), "ben")
        XCTAssertEqual(offer.offeredItems.map(\.title), ["Gitarre"])
        XCTAssertEqual(offer.coverPath, "x.jpg")
    }

    func testEncodesSnakeCase() throws {
        let body = CreateOfferInput(targetItemId: "t", offeredItemIds: ["o"], requestedItemIds: [], message: nil)
        let json = try XCTUnwrap(String(data: APIClient.encoder.encode(body), encoding: .utf8))
        XCTAssertTrue(json.contains("\"target_item_id\""))
        XCTAssertTrue(json.contains("\"offered_item_ids\""))
    }

    func testServerErrorsAreTranslated() {
        let error = APIError.server(status: 409, code: "no_active_item", message: "english")
        XCTAssertEqual(error.errorDescription, "Du brauchst mindestens einen aktiven Gegenstand, um ein Angebot zu machen.")
        let unknown = APIError.server(status: 409, code: "brand_new", message: "fallback")
        XCTAssertEqual(unknown.errorDescription, "fallback")
    }
}

final class ImageCompressorTests: XCTestCase {
    func testLimitsLongestEdgeTo1600() {
        XCTAssertEqual(ImageCompressor.targetSize(for: CGSize(width: 4032, height: 3024)), CGSize(width: 1600, height: 1200))
        XCTAssertEqual(ImageCompressor.targetSize(for: CGSize(width: 1000, height: 3200)), CGSize(width: 500, height: 1600))
        XCTAssertEqual(ImageCompressor.targetSize(for: CGSize(width: 800, height: 600)), CGSize(width: 800, height: 600))
    }

    func testProducesJPEG() throws {
        let image = UIGraphicsImageRenderer(size: CGSize(width: 3000, height: 2000)).image { ctx in
            UIColor.systemTeal.setFill()
            ctx.fill(CGRect(x: 0, y: 0, width: 3000, height: 2000))
        }
        let data = try XCTUnwrap(ImageCompressor.jpeg(from: image))
        XCTAssertEqual(Array(data.prefix(3)), [0xFF, 0xD8, 0xFF])
        let decoded = try XCTUnwrap(UIImage(data: data))
        XCTAssertEqual(decoded.size.width * decoded.scale, 1600)
    }
}

final class FormattingTests: XCTestCase {
    func testExpiryRoundsUp() {
        let now = Date(timeIntervalSince1970: 0)
        XCTAssertEqual(expiryText(now.addingTimeInterval(14 * 86400 - 1), now: now), "läuft in 14 Tagen ab")
        XCTAssertEqual(expiryText(now.addingTimeInterval(20 * 3600), now: now), "läuft in 20 Stunden ab")
        XCTAssertEqual(expiryText(now.addingTimeInterval(-5), now: now), "abgelaufen")
    }
}
