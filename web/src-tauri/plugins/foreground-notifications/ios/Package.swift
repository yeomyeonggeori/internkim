// swift-tools-version:5.5

import PackageDescription

let package = Package(
    name: "tauri-plugin-foreground-notifications",
    platforms: [
        .macOS(.v10_13),
        .iOS(.v15),
    ],
    products: [
        .library(
            name: "tauri-plugin-foreground-notifications",
            type: .static,
            targets: ["tauri-plugin-foreground-notifications"]),
    ],
    dependencies: [
        .package(name: "Tauri", path: "../.tauri/tauri-api")
    ],
    targets: [
        .target(
            name: "tauri-plugin-foreground-notifications",
            dependencies: [
                .byName(name: "Tauri")
            ],
            path: "Sources")
    ]
)
