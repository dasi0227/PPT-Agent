package com.dasi.ppt.agent.infrastructure.persistence.po;

import lombok.AllArgsConstructor;
import lombok.Builder;
import lombok.Getter;
import lombok.NoArgsConstructor;
import lombok.Setter;

@Builder
@Getter
@Setter
@NoArgsConstructor
@AllArgsConstructor
public class FileOpenSettingsPo {

    private int id;
    private String defaultOpen;
    private String jsonOpen;
    private String htmlOpen;
    private String customApps;
}
