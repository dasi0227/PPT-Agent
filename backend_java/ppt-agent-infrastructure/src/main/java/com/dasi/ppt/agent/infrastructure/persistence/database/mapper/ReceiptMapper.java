package com.dasi.ppt.agent.infrastructure.persistence.database.mapper;

import com.dasi.ppt.agent.infrastructure.persistence.database.po.ReceiptPo;
import org.apache.ibatis.annotations.Param;

public interface ReceiptMapper {

    void deleteByProject(String projectId);

    void insertManualReceipt(ReceiptPo receipt);

    String findManualRequestHash(@Param("projectId") String projectId, @Param("operationId") String operationId);

    void deleteManualReceipt(@Param("projectId") String projectId, @Param("operationId") String operationId);
}
