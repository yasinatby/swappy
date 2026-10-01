import Foundation

/// Errors surfaced to the UI. Server errors carry the backend's stable code.
enum APIError: LocalizedError, Equatable {
    case server(status: Int, code: String, message: String)
    case unauthorized
    case transport(String)
    case decoding(String)

    var errorDescription: String? {
        switch self {
        case .unauthorized:
            return "Deine Sitzung ist abgelaufen. Bitte melde dich erneut an."
        case .transport:
            return "Der Server ist nicht erreichbar. Läuft das Backend?"
        case .decoding:
            return "Die Antwort des Servers war unerwartet."
        case let .server(_, code, message):
            return Self.germanMessages[code] ?? message
        }
    }

    static let germanMessages: [String: String] = [
        "no_active_item": "Du brauchst mindestens einen aktiven Gegenstand, um ein Angebot zu machen.",
        "item_unavailable": "Mindestens ein Gegenstand ist nicht mehr verfügbar.",
        "offer_not_pending": "Dieses Angebot ist nicht mehr offen.",
        "offer_not_accepted": "Dieses Angebot wurde noch nicht angenommen.",
        "offer_expired": "Dieses Angebot ist abgelaufen.",
        "blocked": "Mit dieser Person kannst du nicht tauschen.",
        "already_confirmed": "Du hast die Übergabe bereits bestätigt.",
        "item_locked": "Der Gegenstand steckt in einem laufenden Tausch und kann nicht geändert werden.",
        "too_many_images": "Maximal 6 Fotos pro Gegenstand.",
        "display_name_taken": "Dieser Name ist schon vergeben.",
        "item_not_found": "Gegenstand nicht gefunden.",
        "offer_not_found": "Angebot nicht gefunden.",
        "profile_not_found": "Profil nicht gefunden.",
        "not_allowed": "Das darfst du hier nicht.",
        "invalid_image": "Das Bild konnte nicht verarbeitet werden.",
        "image_too_large": "Das Bild ist zu groß.",
    ]
}

/// Thin JSON-over-HTTP client. Knows nothing about screens.
final class APIClient: @unchecked Sendable {
    var baseURL: URL
    var token: String?
    /// Called on any 401 so the session can return to the login screen.
    var onUnauthorized: (@Sendable () -> Void)?
    private let session: URLSession

    init(baseURL: URL, token: String? = nil, session: URLSession = .shared) {
        self.baseURL = baseURL
        self.token = token
        self.session = session
    }

    static let decoder: JSONDecoder = {
        let decoder = JSONDecoder()
        decoder.keyDecodingStrategy = .convertFromSnakeCase
        decoder.dateDecodingStrategy = .custom { decoder in
            let container = try decoder.singleValueContainer()
            let string = try container.decode(String.self)
            guard let date = DateParsing.parse(string) else {
                throw DecodingError.dataCorruptedError(in: container, debugDescription: "Invalid date \(string)")
            }
            return date
        }
        return decoder
    }()

    static let encoder: JSONEncoder = {
        let encoder = JSONEncoder()
        encoder.keyEncodingStrategy = .convertToSnakeCase
        return encoder
    }()

    func imageURL(_ path: String) -> URL {
        baseURL.appending(path: "v1/images/\(path)")
    }

    func get<T: Decodable>(_ path: String, query: [String: String] = [:]) async throws -> T {
        try await decode(send("GET", path, query: query))
    }

    func post<T: Decodable>(_ path: String, body: some Encodable) async throws -> T {
        try await decode(send("POST", path, body: Self.encoder.encode(body)))
    }

    func post<T: Decodable>(_ path: String) async throws -> T {
        try await decode(send("POST", path))
    }

    func put<T: Decodable>(_ path: String, body: some Encodable) async throws -> T {
        try await decode(send("PUT", path, body: Self.encoder.encode(body)))
    }

    /// For endpoints answering 204.
    func perform(_ method: String, _ path: String) async throws {
        _ = try await send(method, path)
    }

    func perform(_ method: String, _ path: String, body: some Encodable) async throws {
        _ = try await send(method, path, body: Self.encoder.encode(body))
    }

    func upload<T: Decodable>(_ path: String, jpeg: Data) async throws -> T {
        try await decode(send("POST", path, body: jpeg, contentType: "image/jpeg"))
    }

    private func send(_ method: String, _ path: String, query: [String: String] = [:],
                      body: Data? = nil, contentType: String = "application/json") async throws -> Data {
        var components = URLComponents(url: baseURL.appending(path: path), resolvingAgainstBaseURL: false)!
        let items = query.filter { !$0.value.isEmpty }.map { URLQueryItem(name: $0.key, value: $0.value) }
        if !items.isEmpty { components.queryItems = items }

        var request = URLRequest(url: components.url!)
        request.httpMethod = method
        request.timeoutInterval = 20
        if let body {
            request.httpBody = body
            request.setValue(contentType, forHTTPHeaderField: "Content-Type")
        }
        if let token {
            request.setValue("Bearer \(token)", forHTTPHeaderField: "Authorization")
        }

        let data: Data
        let response: URLResponse
        do {
            (data, response) = try await session.data(for: request)
        } catch {
            if (error as? URLError)?.code == .cancelled { throw CancellationError() }
            throw APIError.transport(error.localizedDescription)
        }
        guard let http = response as? HTTPURLResponse else {
            throw APIError.transport("no HTTP response")
        }
        if http.statusCode == 401 {
            onUnauthorized?()
            throw APIError.unauthorized
        }
        guard (200 ..< 300).contains(http.statusCode) else {
            let envelope = try? Self.decoder.decode(ErrorEnvelope.self, from: data)
            throw APIError.server(status: http.statusCode,
                                  code: envelope?.error.code ?? "http_\(http.statusCode)",
                                  message: envelope?.error.message ?? "HTTP \(http.statusCode)")
        }
        return data
    }

    private func decode<T: Decodable>(_ data: Data) throws -> T {
        do {
            return try Self.decoder.decode(T.self, from: data)
        } catch {
            throw APIError.decoding(String(describing: error))
        }
    }

    private struct ErrorEnvelope: Decodable {
        struct Body: Decodable {
            let code: String
            let message: String
        }
        let error: Body
    }
}

/// Go encodes timestamps as RFC 3339 with up to nanosecond precision, which
/// ISO8601DateFormatter does not accept, so fractions are cut to milliseconds.
enum DateParsing {
    private static let withFraction: ISO8601DateFormatter = {
        let formatter = ISO8601DateFormatter()
        formatter.formatOptions = [.withInternetDateTime, .withFractionalSeconds]
        return formatter
    }()

    private static let withoutFraction: ISO8601DateFormatter = {
        let formatter = ISO8601DateFormatter()
        formatter.formatOptions = [.withInternetDateTime]
        return formatter
    }()

    static func parse(_ string: String) -> Date? {
        if let date = withoutFraction.date(from: string) { return date }
        guard let dot = string.firstIndex(of: ".") else { return nil }
        let fractionStart = string.index(after: dot)
        let fractionEnd = string[fractionStart...].firstIndex { !$0.isNumber } ?? string.endIndex
        let digits = string[fractionStart ..< fractionEnd].prefix(3)
        let normalized = string[..<fractionStart] + digits.padding(toLength: 3, withPad: "0", startingAt: 0) + string[fractionEnd...]
        return withFraction.date(from: String(normalized))
    }
}
