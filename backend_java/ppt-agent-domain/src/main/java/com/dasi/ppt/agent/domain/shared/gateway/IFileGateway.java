package com.dasi.ppt.agent.domain.shared.gateway;

import java.nio.file.Path;
import java.util.List;

public interface IFileGateway {

    /** 返回直属普通文件名，目录缺失时返回空列表。 */
    List<String> listFileNames(Path path);

    /** 文件缺失时返回 null。 */
    byte[] readFile(Path path);

    /** 完整写入后才发布文件，目标已存在时失败。 */
    void createFile(Path path, byte[] content);

    /** 原子写入完整内容，目标可以是新文件或已有文件。 */
    void replaceFile(Path path, byte[] content);

    /** 删除文件，目标已缺失时直接成功。 */
    void deleteFile(Path path);
}
