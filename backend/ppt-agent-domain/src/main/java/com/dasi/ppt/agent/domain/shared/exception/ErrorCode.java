package com.dasi.ppt.agent.domain.shared.exception;

import com.fasterxml.jackson.annotation.JsonCreator;
import com.fasterxml.jackson.annotation.JsonValue;
import lombok.AllArgsConstructor;
import lombok.Getter;

@Getter
@AllArgsConstructor
public enum ErrorCode {
    INVALID_ARGUMENT("INVALID_ARGUMENT", "请求字段缺失、类型或取值不正确"),
    ILLEGAL_STATE("ILLEGAL_STATE", "已保存的数据状态无效"),
    PROJECT_NOT_FOUND("PROJECT_NOT_FOUND", "项目不存在"),
    SESSION_NOT_FOUND("SESSION_NOT_FOUND", "会话不存在"),
    ASSET_NOT_FOUND("ASSET_NOT_FOUND", "资产不存在"),
    MODEL_NOT_FOUND("MODEL_NOT_FOUND", "模型配置不存在"),
    MODEL_IN_USE("MODEL_IN_USE", "路由正在使用该模型，无法删除"),
    FILE_NOT_FOUND("FILE_NOT_FOUND", "项目文件不存在"),
    FILE_OPEN_FAILED("FILE_OPEN_FAILED", "无法打开文件，请确认应用已安装或在设置中更换打开方式"),
    APPLICATION_PICK_FAILED("APPLICATION_PICK_FAILED", "无法打开应用选择器，请确认桌面环境可用"),
    TAG_NOT_FOUND("TAG_NOT_FOUND", "标签不存在"),
    ASSET_IN_USE("ASSET_IN_USE", "项目正在使用该主题，无法删除"),
    INTERNAL_ERROR("INTERNAL_ERROR", "读取或保存数据失败"),
    IDEMPOTENCY_CONFLICT("IDEMPOTENCY_CONFLICT", "该请求标识已用于其他请求"),
    CONTENT_CONFLICT("CONTENT_CONFLICT", "内容已更新，请重新读取后保存"),
    PROJECT_BUSY("PROJECT_BUSY", "项目正在执行其他操作，请稍后重试"),
    SESSION_BUSY("SESSION_BUSY", "会话正在执行其他操作，请稍后重试"),
    RUN_NOT_FOUND("RUN_NOT_FOUND", "运行记录不存在"),
    ACTION_NOT_FOUND("COMMAND_NOT_FOUND", "命令记录不存在"),
    STALE_BASE("STALE_BASE", "修订基准已过期"),
    STALE_REVISION("STALE_REVISION", "版本已过期，请刷新后重试"),
    INTERACTION_CLOSED("INTERACTION_CLOSED", "交互事项已关闭或已答复"),
    STALE_INTERACTION("STALE_INTERACTION", "待答内容已更新，请刷新后重新提交"),
    BRANCH_CONFIRMATION_REQUIRED("BRANCH_CONFIRMATION_REQUIRED", "请先确认分支操作");

    @JsonValue
    private final String value;
    private final String defaultMessage;

    @JsonCreator
    public static ErrorCode fromValue(String value) {
        for (ErrorCode candidate : values()) {
            if (candidate.value.equals(value)) {
                return candidate;
            }
        }
        throw new InvalidEnumValueException(ErrorCode.class, value);
    }
}
