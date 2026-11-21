package com.dasi.ppt.agent.infrastructure.persistence.database.po;

import lombok.Getter;
import lombok.Setter;

@Getter
@Setter
public class ShortcutSettingsPo {

    private int id;
    private String commandTrigger;
    private String referenceTrigger;
    private String componentTrigger;
    private String slideTrigger;
    private String snippetTrigger;
    private String previousSlide;
    private String nextSlide;
    private String zoomIn;
    private String zoomOut;
    private String toggleOverview;
    private String exportDeck;
    private String toggleView;
    private String toggleSource;
    private String switchTheme;
    private String submitRequest;
    private String suggestion1;
    private String suggestion2;
    private String suggestion3;
}
