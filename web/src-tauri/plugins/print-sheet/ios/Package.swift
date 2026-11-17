// swift-tools-version:5.5

import PackageDescription

let package = Package(
    name: "tauri-plugin-print-sheet",
    platforms: [
        .macOS(.v10_13),
        .iOS(.v15),
    ],
    products: [
        .library(
            name: "tauri-plugin-print-sheet",
            type: .static,
            targets: ["tauri-plugin-print-sheet"]),
    ],
    dependencies: [
        .package(name: "Tauri", path: "../.tauri/tauri-api")
    ],
    targets: [
        .target(
            name: "tauri-plugin-print-sheet",
            dependencies: [
                .byName(name: "Tauri")
            ],
            path: "Sources")
    ]
)
