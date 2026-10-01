import UIKit

/// Shrinks photos before upload: longest edge at most 1600 px, JPEG quality
/// 0.7 (AGENTS.md §7).
enum ImageCompressor {
    static let maxEdge: CGFloat = 1600
    static let quality: CGFloat = 0.7

    static func jpeg(from data: Data) -> Data? {
        guard let image = UIImage(data: data) else { return nil }
        return jpeg(from: image)
    }

    static func jpeg(from image: UIImage) -> Data? {
        let size = targetSize(for: image.size)
        let format = UIGraphicsImageRendererFormat.default()
        format.scale = 1
        format.opaque = true
        let resized = UIGraphicsImageRenderer(size: size, format: format).image { _ in
            image.draw(in: CGRect(origin: .zero, size: size))
        }
        return resized.jpegData(compressionQuality: quality)
    }

    static func targetSize(for size: CGSize) -> CGSize {
        let longest = max(size.width, size.height)
        guard longest > maxEdge, longest > 0 else { return size }
        let scale = maxEdge / longest
        return CGSize(width: (size.width * scale).rounded(), height: (size.height * scale).rounded())
    }
}
