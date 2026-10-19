// swift-tools-version:5.5

import PackageDescription

let package = Package(
    name: "tauri-plugin-page-theme",
    platforms: [
        .macOS(.v10_13),
        .iOS(.v15),
    ],
    products: [
        .library(
            name: "tauri-plugin-page-theme",
            type: .static,
            targets: ["tauri-plugin-page-theme"]),
    ],
    dependencies: [
        .package(name: "Tauri", path: "../.tauri/tauri-api")
    ],
    targets: [
        .target(
            name: "tauri-plugin-page-theme",
            dependencies: [
                .byName(name: "Tauri")
            ],
            path: "Sources")
    ]
)
