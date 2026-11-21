package com.dasi.ppt.agent.infrastructure.persistence.database.po;

import lombok.Getter;
import lombok.Setter;

@Getter
@Setter
public class FileOpenSettingsPo {

    private int id;
    private String defaultOpen;
    private String jsonOpen;
    private String htmlOpen;
    private String customApps;
}
