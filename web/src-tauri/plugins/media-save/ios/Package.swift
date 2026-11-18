// swift-tools-version:5.5

import PackageDescription

let package = Package(
    name: "tauri-plugin-media-save",
    platforms: [
        .macOS(.v10_13),
        .iOS(.v15),
    ],
    products: [
        .library(
            name: "tauri-plugin-media-save",
            type: .static,
            targets: ["tauri-plugin-media-save"]),
    ],
    dependencies: [
        .package(name: "Tauri", path: "../.tauri/tauri-api")
    ],
    targets: [
        .target(
            name: "tauri-plugin-media-save",
            dependencies: [
                .byName(name: "Tauri")
            ],
            path: "Sources")
    ]
)
