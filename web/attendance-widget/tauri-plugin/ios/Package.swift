// swift-tools-version:5.5

import PackageDescription

let package = Package(
    name: "tauri-plugin-attendance-widget",
    platforms: [
        .macOS(.v10_13),
        .iOS(.v15),
    ],
    products: [
        .library(
            name: "tauri-plugin-attendance-widget",
            type: .static,
            targets: ["tauri-plugin-attendance-widget"]),
    ],
    dependencies: [
        .package(name: "Tauri", path: "../.tauri/tauri-api")
    ],
    targets: [
        .target(
            name: "tauri-plugin-attendance-widget",
            dependencies: [
                .byName(name: "Tauri")
            ],
            path: "Sources")
    ]
)
