// swift-tools-version:5.5

import PackageDescription

let package = Package(
    name: "tauri-plugin-silent-push",
    platforms: [
        .macOS(.v10_13),
        .iOS(.v15),
    ],
    products: [
        .library(
            name: "tauri-plugin-silent-push",
            type: .static,
            targets: ["tauri-plugin-silent-push"]),
    ],
    dependencies: [
        .package(name: "Tauri", path: "../.tauri/tauri-api")
    ],
    targets: [
        .target(
            name: "tauri-plugin-silent-push",
            dependencies: [
                .byName(name: "Tauri")
            ],
            path: "Sources")
    ]
)
