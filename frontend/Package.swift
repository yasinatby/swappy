// swift-tools-version: 6.0
import PackageDescription

let package = Package(
    name: "SwappyFrontend",
    platforms: [
        .macOS(.v13),
        .iOS(.v16)
    ],
    products: [
        .library(
            name: "SwappyClient",
            targets: ["SwappyClient"]
        ),
        .executable(
            name: "SwappyTestRunner",
            targets: ["SwappyTestRunner"]
        )
    ],
    targets: [
        .target(
            name: "SwappyClient",
            path: "Sources/SwappyClient"
        ),
        .executableTarget(
            name: "SwappyTestRunner",
            dependencies: ["SwappyClient"],
            path: "Sources/SwappyTestRunner"
        ),
        .testTarget(
            name: "SwappyClientTests",
            dependencies: ["SwappyClient"],
            path: "Tests/SwappyClientTests"
        )
    ]
)
