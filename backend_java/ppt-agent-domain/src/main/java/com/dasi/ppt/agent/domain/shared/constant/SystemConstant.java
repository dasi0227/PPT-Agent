package com.dasi.ppt.agent.domain.shared.constant;

public final class SystemConstant {

    private static final String OS_NAME = System.getProperty("os.name");
    public static final boolean IS_MAC = OS_NAME.startsWith("Mac");
    public static final boolean IS_WINDOWS = OS_NAME.startsWith("Windows");
    public static final String APPLICATION_SUFFIX = IS_WINDOWS ? ".exe" : ".app";

    public static final String SYSTEM_OPEN = "system";
    public static final String INHERIT_OPEN = "inherit";
}
