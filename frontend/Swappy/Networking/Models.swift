import Foundation

// Wire models. JSON keys are snake_case and decoded via .convertFromSnakeCase,
// so `owner_id` becomes `ownerId`. There is deliberately no price anywhere.

struct Profile: Codable, Identifiable, Hashable {
    let id: String
    let displayName: String
    let postalCode: String?
    let createdAt: Date
    let activeItems: Int
    let completedTrades: Int
    let distanceKm: Int?
}

struct ItemImage: Codable, Identifiable, Hashable {
    let id: String
    let path: String
    let position: Int
}

struct Item: Codable, Identifiable, Hashable {
    let id: String
    let ownerId: String
    let ownerName: String
    let title: String
    let description: String?
    let category: String
    let condition: ItemCondition
    let wantedText: String?
    let status: ItemStatus
    let coverPath: String?
    let distanceKm: Int?
    let createdAt: Date
    let updatedAt: Date
    let images: [ItemImage]?
}

enum ItemStatus: String, Codable, Hashable {
    case draft, active, reserved, traded, removed, deleted

    var label: String {
        switch self {
        case .draft: "Entwurf"
        case .active: "Aktiv"
        case .reserved: "Reserviert"
        case .traded: "Getauscht"
        case .removed: "Entfernt"
        case .deleted: "Gelöscht"
        }
    }

    var isEditable: Bool { self == .draft || self == .active }
}

enum ItemCondition: String, Codable, CaseIterable, Hashable, Identifiable {
    case new, good, used
    var id: String { rawValue }

    var label: String {
        switch self {
        case .new: "Neu"
        case .good: "Gut"
        case .used: "Gebraucht"
        }
    }
}

struct ItemInput: Encodable {
    var title: String
    var description: String?
    var category: String
    var condition: ItemCondition
    var wantedText: String?
}

struct OfferItem: Codable, Identifiable, Hashable {
    let id: String
    let side: OfferSide
    let title: String
    let status: ItemStatus
    let coverPath: String?
}

enum OfferSide: String, Codable, Hashable {
    case offered, requested
}

enum OfferStatus: String, Codable, Hashable {
    case pending, accepted, declined, cancelled, expired, completed

    var label: String {
        switch self {
        case .pending: "Offen"
        case .accepted: "Angenommen"
        case .declined: "Abgelehnt"
        case .cancelled: "Storniert"
        case .expired: "Abgelaufen"
        case .completed: "Getauscht"
        }
    }

    var isOpen: Bool { self == .pending || self == .accepted }
}

struct Offer: Codable, Identifiable, Hashable {
    let id: String
    let initiatorId: String
    let responderId: String
    let targetItemId: String
    let message: String?
    let status: OfferStatus
    let expiresAt: Date
    let acceptedAt: Date?
    let initiatorConfirmedAt: Date?
    let responderConfirmedAt: Date?
    let createdAt: Date
    let initiatorName: String
    let responderName: String
    let items: [OfferItem]

    var offeredItems: [OfferItem] { items.filter { $0.side == .offered } }
    var requestedItems: [OfferItem] { items.filter { $0.side == .requested } }

    func isIncoming(for userId: String) -> Bool { responderId == userId }

    func counterpartyName(for userId: String) -> String {
        initiatorId == userId ? responderName : initiatorName
    }

    func counterpartyId(for userId: String) -> String {
        initiatorId == userId ? responderId : initiatorId
    }

    func hasConfirmed(_ userId: String) -> Bool {
        userId == initiatorId ? initiatorConfirmedAt != nil : responderConfirmedAt != nil
    }

    var coverPath: String? {
        (items.first { $0.id == targetItemId } ?? items.first)?.coverPath
    }
}

struct CreateOfferInput: Encodable {
    let targetItemId: String
    let offeredItemIds: [String]
    let requestedItemIds: [String]
    let message: String?
}

enum OfferAction: String {
    case accept, decline, cancel, confirm
}

struct AppNotification: Codable, Identifiable, Hashable {
    let id: String
    let kind: String
    let offerId: String?
    let createdAt: Date
    let readAt: Date?

    var text: String {
        switch kind {
        case "offer_received": "Du hast ein neues Tauschangebot erhalten."
        case "offer_accepted": "Dein Angebot wurde angenommen."
        case "offer_declined": "Dein Angebot wurde abgelehnt."
        case "offer_cancelled": "Ein Tausch wurde storniert."
        case "offer_expired": "Ein Angebot ist abgelaufen."
        case "trade_confirmed": "Die Gegenseite hat die Übergabe bestätigt."
        case "trade_completed": "Tausch abgeschlossen!"
        default: "Neuigkeit zu einem Tausch"
        }
    }

    var symbol: String {
        switch kind {
        case "offer_received": "tray.and.arrow.down"
        case "offer_accepted": "checkmark.circle"
        case "offer_declined", "offer_cancelled": "xmark.circle"
        case "offer_expired": "clock"
        case "trade_confirmed": "hand.thumbsup"
        case "trade_completed": "arrow.left.arrow.right.circle.fill"
        default: "bell"
        }
    }
}

struct BlockedUser: Codable, Identifiable, Hashable {
    let id: String
    let displayName: String
    let blockedAt: Date
}

enum ReportReason: String, CaseIterable, Identifiable, Encodable {
    case spam, prohibitedItem = "prohibited_item", offensive, scam, other
    var id: String { rawValue }

    var label: String {
        switch self {
        case .spam: "Spam"
        case .prohibitedItem: "Verbotener Gegenstand"
        case .offensive: "Beleidigend / unangemessen"
        case .scam: "Betrugsverdacht"
        case .other: "Sonstiges"
        }
    }
}

enum ReportSubject: String, Encodable {
    case item, profile
}

/// Fixed category list; must match items.Categories in the backend.
struct Category: Identifiable, Hashable {
    let id: String
    let label: String
    let symbol: String

    static let all: [Category] = [
        .init(id: "elektronik", label: "Elektronik", symbol: "desktopcomputer"),
        .init(id: "haushalt", label: "Haushalt", symbol: "cup.and.saucer"),
        .init(id: "moebel", label: "Möbel", symbol: "sofa"),
        .init(id: "kleidung", label: "Kleidung", symbol: "tshirt"),
        .init(id: "buecher", label: "Bücher", symbol: "book"),
        .init(id: "musik", label: "Musik", symbol: "guitars"),
        .init(id: "sport", label: "Sport", symbol: "bicycle"),
        .init(id: "spiele", label: "Spiele", symbol: "gamecontroller"),
        .init(id: "kinder", label: "Kinder", symbol: "teddybear"),
        .init(id: "garten", label: "Garten", symbol: "leaf"),
        .init(id: "werkzeug", label: "Werkzeug", symbol: "hammer"),
        .init(id: "sonstiges", label: "Sonstiges", symbol: "shippingbox"),
    ]

    static func label(for id: String) -> String {
        all.first { $0.id == id }?.label ?? id
    }

    static func symbol(for id: String) -> String {
        all.first { $0.id == id }?.symbol ?? "shippingbox"
    }
}

struct DevLoginResponse: Decodable {
    let token: String
    let profile: Profile
}

struct ProfilesResponse: Decodable { let profiles: [Profile] }
struct ItemsResponse: Decodable { let items: [Item] }
struct OffersResponse: Decodable { let offers: [Offer] }
struct NotificationsResponse: Decodable { let notifications: [AppNotification] }
struct BlockedResponse: Decodable { let blocked: [BlockedUser] }
