package com.dasi.ppt.agent.domain.agent.model;

import com.dasi.ppt.agent.domain.project.model.enumeration.DecorationPosition;
import com.dasi.ppt.agent.domain.project.model.enumeration.DecorationType;

import java.util.List;

public record DomSelection(
        String selectionId,
        int markerNo,
        String slideId,
        String html,
        String htmlHash,
        String comment,
        Region region
) {

    public record Region(
            String type,
            Rect rect,
            List<Target> targets,
            List<Decoration> decorations
    ) {
    }

    public record Rect(
            double x,
            double y,
            double width,
            double height
    ) {
    }

    public record Target(
            String targetId,
            String tag,
            Fingerprint fingerprint,
            Rect rect
    ) {
    }

    public record Fingerprint(
            String tag,
            int siblingIndex,
            String stableId,
            List<String> classes,
            List<Ancestor> ancestors
    ) {
    }

    public record Ancestor(
            String tag,
            String stableId,
            String classSummary,
            int siblingIndex
    ) {
    }

    public record Decoration(
            DecorationType type,
            DecorationPosition placement,
            String text,
            Rect rect
    ) {
    }
}
