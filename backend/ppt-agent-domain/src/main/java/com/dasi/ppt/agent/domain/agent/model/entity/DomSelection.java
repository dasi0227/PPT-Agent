package com.dasi.ppt.agent.domain.agent.model.entity;

import com.dasi.ppt.agent.domain.project.model.enumeration.DecorationPosition;
import com.dasi.ppt.agent.domain.project.model.enumeration.DecorationType;
import lombok.AllArgsConstructor;
import lombok.Builder;
import lombok.Getter;
import lombok.NoArgsConstructor;

import java.util.List;

@Builder
@Getter
@AllArgsConstructor
@NoArgsConstructor
public final class DomSelection {
    private String selectionId;
    private int markerNo;
    private String slideId;
    private String html;
    private String htmlHash;
    private String comment;
    private Region region;

    @Builder
    @Getter
    @AllArgsConstructor
    @NoArgsConstructor
    public static final class Region {
        private String type;
        private Rect rect;
        private List<Target> targets;
        private List<Decoration> decorations;
    }

    @Builder
    @Getter
    @AllArgsConstructor
    @NoArgsConstructor
    public static final class Rect {
        private double x;
        private double y;
        private double width;
        private double height;
    }

    @Builder
    @Getter
    @AllArgsConstructor
    @NoArgsConstructor
    public static final class Target {
        private String targetId;
        private String tag;
        private Fingerprint fingerprint;
        private Rect rect;
    }

    @Builder
    @Getter
    @AllArgsConstructor
    @NoArgsConstructor
    public static final class Fingerprint {
        private String tag;
        private int siblingIndex;
        private String stableId;
        private List<String> classes;
        private List<Ancestor> ancestors;
    }

    @Builder
    @Getter
    @AllArgsConstructor
    @NoArgsConstructor
    public static final class Ancestor {
        private String tag;
        private String stableId;
        private String classSummary;
        private int siblingIndex;
    }

    @Builder
    @Getter
    @AllArgsConstructor
    @NoArgsConstructor
    public static final class Decoration {
        private DecorationType type;
        private DecorationPosition placement;
        private String text;
        private Rect rect;
    }
}
