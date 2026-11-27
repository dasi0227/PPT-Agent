package com.dasi.ppt.agent.domain.shared.constant;

public final class SystemConstant {

    private static final String OS_NAME = System.getProperty("os.name");
    public static final boolean IS_MAC = OS_NAME.startsWith("Mac");
    public static final boolean IS_WINDOWS = OS_NAME.startsWith("Windows");
    public static final String APPLICATION_SUFFIX = IS_WINDOWS ? ".exe" : ".app";

    public static final String META_FILE_NAME = ".meta.json";
    public static final String CONSTRAINT_FILE_NAME = ".constraint.json";
    public static final String OUTLINE_FILE_NAME = ".outline.json";

    public static final String SLIDE_SPEC_FILE_NAME_FORMAT = "%s.json";
    public static final String SLIDE_HTML_FILE_NAME_FORMAT = "%s.html";

    private static final String RUNTIME_DIR = ".runtime/";
    public static final String ATTACHMENT_PATH_PREFIX = RUNTIME_DIR + "attachments/";
    public static final String SCREENSHOT_PATH_FORMAT = RUNTIME_DIR + "screenshots/%s.png";
    public static final String THUMBNAIL_DIRECTORY_FORMAT = RUNTIME_DIR + "thumbnails/%s";

    public static final String SYSTEM_OPEN = "system";
    public static final String INHERIT_OPEN = "inherit";
}
