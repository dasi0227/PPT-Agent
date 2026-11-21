package com.dasi.ppt.agent.infrastructure.persistence.database.po;

import lombok.Getter;
import lombok.Setter;

@Getter
@Setter
public class ReceiptPo {

    private String type;
    private String ownerId;
    private String requestId;
    private String requestHash;
    private String result;
}
