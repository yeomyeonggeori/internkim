package app.intern.kim.attendance;

import androidx.annotation.Nullable;

final class WorkLocation {

    final String name;

    @Nullable
    final String color;

    WorkLocation(String name, @Nullable String color) {
        this.name = name;
        this.color = color;
    }
}
